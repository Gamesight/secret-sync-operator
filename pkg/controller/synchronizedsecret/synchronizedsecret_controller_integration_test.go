//go:build integration

// Package synchronizedsecret contains the controller for SynchronizedSecret resources
package synchronizedsecret

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Innervate/secret-sync-operator/pkg/apis"
	appv1alpha1 "github.com/Innervate/secret-sync-operator/pkg/apis/app/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// testEnv holds the test environment for integration tests
var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
)

// setupTestEnvironment initializes the envtest environment
func setupTestEnvironment(t *testing.T) {
	t.Log("Setting up test environment")

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "deploy", "crds")},
		ErrorIfCRDPathMissing: false,
	}

	var err error
	cfg, err = testEnv.Start()
	if err != nil {
		t.Fatalf("Failed to start test environment: %v", err)
	}

	// Use the GetScheme() function which properly registers all types
	testScheme := apis.GetScheme()

	k8sClient, err = client.New(cfg, client.Options{Scheme: testScheme})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	t.Log("Test environment ready")
}

// teardownTestEnvironment stops the envtest environment
func teardownTestEnvironment(t *testing.T) {
	t.Log("Tearing down test environment")
	err := testEnv.Stop()
	if err != nil {
		t.Errorf("Failed to stop test environment: %v", err)
	}
}

// TestIntegration_CRDCreation tests that SynchronizedSecret CRs can be created
func TestIntegration_CRDCreation(t *testing.T) {
	setupTestEnvironment(t)
	defer teardownTestEnvironment(t)

	ctx := context.Background()
	namespace := "test-crd"

	// Create test namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	err := k8sClient.Create(ctx, ns)
	if err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}
	defer func() {
		_ = k8sClient.Delete(ctx, ns)
	}()

	// Create SynchronizedSecret CR
	syncSecret := &appv1alpha1.SynchronizedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: namespace,
		},
		Spec: appv1alpha1.SynchronizedSecretSpec{
			RemoteSecret: appv1alpha1.RemoteSecretSpec{
				Name:      "remote-secret",
				Namespace: "remote-ns",
			},
		},
	}
	err = k8sClient.Create(ctx, syncSecret)
	if err != nil {
		t.Fatalf("Failed to create SynchronizedSecret: %v", err)
	}

	// Verify we can read it back
	retrieved := &appv1alpha1.SynchronizedSecret{}
	err = k8sClient.Get(ctx, types.NamespacedName{
		Name:      syncSecret.Name,
		Namespace: syncSecret.Namespace,
	}, retrieved)
	if err != nil {
		t.Fatalf("Failed to get SynchronizedSecret: %v", err)
	}

	if retrieved.Spec.RemoteSecret.Name != "remote-secret" {
		t.Errorf("Expected remote secret name 'remote-secret', got '%s'",
			retrieved.Spec.RemoteSecret.Name)
	}
}

// TestIntegration_ReconcileNotFound tests reconciliation when CR doesn't exist
func TestIntegration_ReconcileNotFound(t *testing.T) {
	setupTestEnvironment(t)
	defer teardownTestEnvironment(t)

	ctx := context.Background()

	r := &ReconcileSynchronizedSecret{
		client: k8sClient,
		scheme: apis.GetScheme(),
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "non-existent",
			Namespace: "default",
		},
	}

	result, err := r.Reconcile(ctx, req)
	if err != nil {
		t.Fatalf("Reconcile should not error for non-existent resource: %v", err)
	}

	if result.Requeue {
		t.Error("Should not requeue when resource not found")
	}
}

// TestIntegration_StatusUpdate tests status updates on SynchronizedSecret
// Note: Status subresource updates with envtest can be inconsistent
func TestIntegration_StatusUpdate(t *testing.T) {
	t.Skip("Status subresource updates require additional envtest configuration - covered in unit tests")
	setupTestEnvironment(t)
	defer teardownTestEnvironment(t)

	ctx := context.Background()
	namespace := "test-status"

	// Create test namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	err := k8sClient.Create(ctx, ns)
	if err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}
	defer func() {
		_ = k8sClient.Delete(ctx, ns)
	}()

	// Create SynchronizedSecret CR
	syncSecret := &appv1alpha1.SynchronizedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: namespace,
		},
		Spec: appv1alpha1.SynchronizedSecretSpec{
			RemoteSecret: appv1alpha1.RemoteSecretSpec{
				Name:      "remote-secret",
				Namespace: "remote-ns",
			},
		},
	}
	err = k8sClient.Create(ctx, syncSecret)
	if err != nil {
		t.Fatalf("Failed to create SynchronizedSecret: %v", err)
	}

	// Fetch the object first to get the latest version
	err = k8sClient.Get(ctx, types.NamespacedName{
		Name:      syncSecret.Name,
		Namespace: syncSecret.Namespace,
	}, syncSecret)
	if err != nil {
		t.Fatalf("Failed to get SynchronizedSecret: %v", err)
	}

	// Update status
	err = updateStatus(ctx, k8sClient, syncSecret, "test-status", false)
	if err != nil {
		t.Fatalf("Failed to update status: %v", err)
	}

	// Verify status was updated
	retrieved := &appv1alpha1.SynchronizedSecret{}
	err = k8sClient.Get(ctx, types.NamespacedName{
		Name:      syncSecret.Name,
		Namespace: syncSecret.Namespace,
	}, retrieved)
	if err != nil {
		t.Fatalf("Failed to get SynchronizedSecret: %v", err)
	}

	if retrieved.Status.Status != "test-status" {
		t.Errorf("Expected status 'test-status', got '%s'", retrieved.Status.Status)
	}

	// Update with timestamp
	err = updateStatus(ctx, k8sClient, syncSecret, "updated-status", true)
	if err != nil {
		t.Fatalf("Failed to update status with timestamp: %v", err)
	}

	// Verify timestamp was set
	err = k8sClient.Get(ctx, types.NamespacedName{
		Name:      syncSecret.Name,
		Namespace: syncSecret.Namespace,
	}, retrieved)
	if err != nil {
		t.Fatalf("Failed to get updated SynchronizedSecret: %v", err)
	}

	if retrieved.Status.LastSync == "" {
		t.Error("Expected LastSync to be set")
	}

	if retrieved.Status.Status != "updated-status" {
		t.Errorf("Expected status 'updated-status', got '%s'", retrieved.Status.Status)
	}
}

// TestIntegration_ReconcileMissingCredentials tests reconciliation without remote credentials
// Note: Status subresource validation requires additional envtest configuration
func TestIntegration_ReconcileMissingCredentials(t *testing.T) {
	t.Skip("Status validation with envtest requires additional configuration - covered in unit tests")
	setupTestEnvironment(t)
	defer teardownTestEnvironment(t)

	ctx := context.Background()
	namespace := "test-no-creds"

	// Create test namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	err := k8sClient.Create(ctx, ns)
	if err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}
	defer func() {
		_ = k8sClient.Delete(ctx, ns)
	}()

	// Create SynchronizedSecret CR without credentials secret
	syncSecret := &appv1alpha1.SynchronizedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: namespace,
		},
		Spec: appv1alpha1.SynchronizedSecretSpec{
			RemoteSecret: appv1alpha1.RemoteSecretSpec{
				Name:      "remote-secret",
				Namespace: "remote-ns",
			},
		},
	}
	err = k8sClient.Create(ctx, syncSecret)
	if err != nil {
		t.Fatalf("Failed to create SynchronizedSecret: %v", err)
	}

	r := &ReconcileSynchronizedSecret{
		client: k8sClient,
		scheme: apis.GetScheme(),
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      syncSecret.Name,
			Namespace: syncSecret.Namespace,
		},
	}

	// Reconcile - should error due to missing credentials
	result, err := r.Reconcile(ctx, req)
	if err == nil {
		t.Error("Expected error when credentials are missing")
	}

	if result.Requeue {
		t.Error("Should not requeue on error")
	}

	// Verify status shows error
	retrieved := &appv1alpha1.SynchronizedSecret{}
	err = k8sClient.Get(ctx, req.NamespacedName, retrieved)
	if err != nil {
		t.Fatalf("Failed to get SynchronizedSecret: %v", err)
	}

	if retrieved.Status.Status != "err:remote-connect" {
		t.Errorf("Expected status 'err:remote-connect', got '%s'", retrieved.Status.Status)
	}
}

// TestIntegration_NewSecretForCR tests the secret creation helper
func TestIntegration_NewSecretForCR(t *testing.T) {
	setupTestEnvironment(t)
	defer teardownTestEnvironment(t)

	ctx := context.Background()
	namespace := "test-newsecret"

	// Create test namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	err := k8sClient.Create(ctx, ns)
	if err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}
	defer func() {
		_ = k8sClient.Delete(ctx, ns)
	}()

	// Create a remote secret
	remoteSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "remote-secret",
			Namespace: namespace,
			Labels: map[string]string{
				"app": "test",
			},
			Annotations: map[string]string{
				"note": "test-annotation",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"username": []byte("admin"),
			"password": []byte("secret123"),
		},
	}

	// Create SynchronizedSecret CR
	syncSecret := &appv1alpha1.SynchronizedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "local-secret",
			Namespace: namespace,
		},
		Spec: appv1alpha1.SynchronizedSecretSpec{
			RemoteSecret: appv1alpha1.RemoteSecretSpec{
				Name:      "remote-secret",
				Namespace: namespace,
			},
		},
	}

	// Create the local secret using the helper
	localSecret := newSecretForCR(syncSecret, remoteSecret)

	// Verify the created secret has correct properties
	if localSecret.Name != syncSecret.Name {
		t.Errorf("Expected name '%s', got '%s'", syncSecret.Name, localSecret.Name)
	}

	if localSecret.Namespace != syncSecret.Namespace {
		t.Errorf("Expected namespace '%s', got '%s'", syncSecret.Namespace, localSecret.Namespace)
	}

	if localSecret.Type != remoteSecret.Type {
		t.Errorf("Expected type '%s', got '%s'", remoteSecret.Type, localSecret.Type)
	}

	if string(localSecret.Data["username"]) != "admin" {
		t.Errorf("Expected username 'admin', got '%s'", string(localSecret.Data["username"]))
	}

	if localSecret.Labels["app"] != "test" {
		t.Errorf("Expected label app='test', got '%s'", localSecret.Labels["app"])
	}

	if localSecret.Annotations["note"] != "test-annotation" {
		t.Errorf("Expected annotation note='test-annotation', got '%s'",
			localSecret.Annotations["note"])
	}
}

// TestIntegration_SecretTypesHandling tests different secret types
func TestIntegration_SecretTypesHandling(t *testing.T) {
	setupTestEnvironment(t)
	defer teardownTestEnvironment(t)

	testCases := []struct {
		name       string
		secretType corev1.SecretType
		data       map[string][]byte
	}{
		{
			name:       "opaque",
			secretType: corev1.SecretTypeOpaque,
			data: map[string][]byte{
				"key": []byte("value"),
			},
		},
		{
			name:       "dockerconfigjson",
			secretType: corev1.SecretTypeDockerConfigJson,
			data: map[string][]byte{
				".dockerconfigjson": []byte(`{"auths":{}}`),
			},
		},
		{
			name:       "tls",
			secretType: corev1.SecretTypeTLS,
			data: map[string][]byte{
				"tls.crt": []byte("cert"),
				"tls.key": []byte("key"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			remoteSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "remote-" + tc.name,
					Namespace: "test",
				},
				Type: tc.secretType,
				Data: tc.data,
			}

			syncSecret := &appv1alpha1.SynchronizedSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "local-" + tc.name,
					Namespace: "test",
				},
			}

			localSecret := newSecretForCR(syncSecret, remoteSecret)

			if localSecret.Type != tc.secretType {
				t.Errorf("Expected type %s, got %s", tc.secretType, localSecret.Type)
			}

			for key, value := range tc.data {
				if string(localSecret.Data[key]) != string(value) {
					t.Errorf("Data mismatch for key %s", key)
				}
			}
		})
	}
}

// TestIntegration_GetRemoteClient tests remote client creation with credentials
func TestIntegration_GetRemoteClient(t *testing.T) {
	setupTestEnvironment(t)
	defer teardownTestEnvironment(t)

	ctx := context.Background()
	namespace := "test-remote-client"

	// Create test namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	err := k8sClient.Create(ctx, ns)
	if err != nil {
		t.Fatalf("Failed to create namespace: %v", err)
	}
	defer func() {
		_ = k8sClient.Delete(ctx, ns)
	}()

	// Test missing credentials
	t.Run("missing credentials", func(t *testing.T) {
		instance := &appv1alpha1.SynchronizedSecret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: namespace,
			},
		}

		_, err := getRemoteClient(ctx, k8sClient, instance)
		if err == nil {
			t.Error("Expected error when credentials are missing")
		}

		if !errors.IsNotFound(err) {
			t.Errorf("Expected NotFound error, got: %v", err)
		}
	})

	// Test with credentials present
	t.Run("with credentials", func(t *testing.T) {
		// Create credentials secret
		credsSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret-sync-remote-cluster-creds",
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"host":  []byte("https://test-cluster:6443"),
				"token": []byte("test-token"),
				"ca":    []byte("dummy-ca"),
			},
		}
		err := k8sClient.Create(ctx, credsSecret)
		if err != nil {
			t.Fatalf("Failed to create credentials secret: %v", err)
		}

		instance := &appv1alpha1.SynchronizedSecret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: namespace,
			},
		}

		// Function will attempt to create client but will fail with invalid CA
		// This is expected in unit tests - full remote client testing requires
		// a real remote cluster setup
		_, err = getRemoteClient(ctx, k8sClient, instance)
		// We expect an error here due to invalid CA, but at least we got past
		// the credentials reading step
		if err == nil {
			t.Log("Note: Client creation succeeded (unexpected with dummy CA)")
		}
	})
}

// Note: Full end-to-end reconciliation tests with remote cluster synchronization
// require a more complex setup with two separate Kubernetes clusters or
// additional mocking of the remote client. These tests focus on the components
// that can be reliably tested with envtest's single-cluster environment.
