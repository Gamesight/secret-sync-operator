package synchronizedsecret

import (
	"context"
	"reflect"
	"time"

	appv1alpha1 "github.com/Innervate/secret-sync-operator/pkg/apis/app/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_synchronizedsecret")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new SynchronizedSecret Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileSynchronizedSecret{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("synchronizedsecret-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource SynchronizedSecret
	err = c.Watch(&source.Kind{Type: &appv1alpha1.SynchronizedSecret{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Secrets and requeue the owner SynchronizedSecret
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &appv1alpha1.SynchronizedSecret{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileSynchronizedSecret{}

// ReconcileSynchronizedSecret reconciles a SynchronizedSecret object
type ReconcileSynchronizedSecret struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a SynchronizedSecret object and makes changes based on the state read
// and what is in the SynchronizedSecret.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileSynchronizedSecret) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling SynchronizedSecret")

	// Refresh our secrets every 10 minutes
	updateRate := time.Minute * 10

	// Fetch the SynchronizedSecret instance
	instance := &appv1alpha1.SynchronizedSecret{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Error retrieving SychronizedSecret")
		updateStatus(&r.client, instance, "err:config-read-failed")
		return reconcile.Result{}, err
	}

	// Get connection to our remote cluster
	remoteClient, err := getRemoteClient(&r.client, instance)
	if err != nil {
		reqLogger.Error(err, "Error connecting to remote cluster (credentials should be in 'secret-sync-remote-cluster-creds')")
		updateStatus(&r.client, instance, "err:remote-connect")
		return reconcile.Result{}, err
	}

	// Read the secret from the remote cluster
	remoteSecret := &corev1.Secret{}
	err = remoteClient.Get(context.TODO(), types.NamespacedName{Name: instance.Spec.RemoteSecret.Name, Namespace: instance.Spec.RemoteSecret.Namespace}, remoteSecret)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Remote secret not found", "Secret.Namespace", instance.Spec.RemoteSecret.Namespace, "Secret.Name", instance.Spec.RemoteSecret.Name)
		updateStatus(&r.client, instance, "err:remote-read-failed")
		// Remote secret doesn't exist... requeue to try again
		return reconcile.Result{RequeueAfter: updateRate}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Define our new local Secret object
	secret := newSecretForCR(instance, remoteSecret)

	// Set SynchronizedSecret instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, secret, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this Secret already exists
	found := &corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, found)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Creating a new Secret", "Secret.Namespace", secret.Namespace, "Secret.Name", secret.Name)
			err = r.client.Create(context.TODO(), secret)
			if err != nil {
				return reconcile.Result{}, err
			}

			updateStatus(&r.client, instance, "insync")
			// Secret created successfully - requeue in 10 minutes
			return reconcile.Result{RequeueAfter: updateRate}, nil
		}

		return reconcile.Result{}, err
	} else if !reflect.DeepEqual(found.Data, secret.Data) {
		reqLogger.Info("Updating existing Secret", "Secret.Namespace", secret.Namespace, "Secret.Name", secret.Name)
		err = r.client.Update(context.TODO(), secret)
		if err != nil {
			return reconcile.Result{}, err
		}

		updateStatus(&r.client, instance, "insync")
		// Secret created successfully - requeue in 10 minutes
		return reconcile.Result{RequeueAfter: updateRate}, nil
	}

	// Secret already exists and is up to date - requeue in 10 minutes
	updateStatus(&r.client, instance, "insync")
	reqLogger.Info("Skip reconcile: Secret already up to date", "Secret.Namespace", found.Namespace, "Secret.Name", found.Name)
	return reconcile.Result{RequeueAfter: updateRate}, nil
}

func getRemoteClient(localClient *client.Client, instance *appv1alpha1.SynchronizedSecret) (client.Client, error) {

	// Poll the remote secret
	remoteClusterSecret := &corev1.Secret{}
	err := (*localClient).Get(context.TODO(), types.NamespacedName{Name: "secret-sync-remote-cluster-creds", Namespace: instance.Namespace}, remoteClusterSecret)
	if err != nil {
		return nil, err
	}

	config := &rest.Config{
		Host:        string(remoteClusterSecret.Data["host"]),
		BearerToken: string(remoteClusterSecret.Data["token"]),
		TLSClientConfig: rest.TLSClientConfig{
			CAData: remoteClusterSecret.Data["ca"],
		},
	}
	remoteClient, err := client.New(config, client.Options{})
	if err != nil {
		return nil, err
	}

	return remoteClient, nil
}

func updateStatus(localClient *client.Client, instance *appv1alpha1.SynchronizedSecret, status string) error {
	// Update status.Nodes if needed
	if instance.Status.Status != status {
		instance.Status.Status = status
		instance.Status.LastSync = time.Now().Format(time.RFC3339)

		return (*localClient).Status().Update(context.TODO(), instance)
	}
	return nil
}

// newSecretForCR returns a secret with the same name/namespace as the cr
func newSecretForCR(cr *appv1alpha1.SynchronizedSecret, remoteSecret *corev1.Secret) *corev1.Secret {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Data: remoteSecret.Data,
	}
}
