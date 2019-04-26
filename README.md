# Secret Sync Operator
A no-frills Kubernetes operator for syncing Secrets across clusters.
[![CircleCI](https://circleci.com/gh/Innervate/secret-sync-operator.svg?style=svg)](https://circleci.com/gh/Innervate/secret-sync-operator)

## Overview
This project is a Kubernetes operator design to keep secrets in sync between multiple clusters. It was created specifically for use in multi-region deployments to keep SSL certificates created through letsencrypt HTTP01 challenges up to date. While designed specifically for this use case, the operator is designed to be generic for any case in which you would want to keep Secrets in sync between clusters or even between namespaces within a cluster.

An example deployment where you have one primary cluster that acts as the source of truth for your secret might look like this.
```
           +-----------------+
           | Primary Cluster |
           +-----------------+
             | Secret Sync |
             V             V
+------------------+    +------------------+
| Regional Cluster |    | Regional Cluster |
+------------------+    +------------------+
```
In this example each of the Regional Clusters run the Secret Sync Operator to regularly update their local copy of the secret from the Primary Cluster.

## Quick Start (with Kustomize)
```
# Create a namespace to run secret-sync-operator in (or use an existing one)
$ kubectl create namespace secret-sync-operator

# Create a new Kustomization file
$ cat << EOF > kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: kube-system

bases:
- github.com/Innervate/secret-sync-operator//deploy
EOF

# Deploy the operator + CustomResourceDefinition
$ kustomize build  | kubectl apply -f -
```

## Connecting to a Remote Cluster
Now that the operator is deployed you can configure the credentials for the remote cluster that you are
synchronizing credentials from.

First we will create a service account that just has access to read the secret we want to sync. This ServiceAccount should be created in the remote cluster (eg the one that you want to pull data from).
```
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: secret-sync-operator-agent
---
kind: Role
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: secret-sync-operator-role
rules:
- apiGroups: [""]
  resources: ["secrets"]
  resourceNames: ["my-secret-to-distribute", "my-secret-to-distribute2"]
  verbs: ["get"]
---
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: secret-sync-operator-agent
subjects:
- kind: ServiceAccount
  name: secret-sync-operator-agent
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: secret-sync-operator-role
```

Now we will pull the credentials for our new service account so we can grant the Secret Sync Operator access to read our remote secrets.
```
$ kubectl get secret secret-sync-operator-agent-token-xxxxx -o "jsonpath={.data.token}"
```

Now create a new secret in the cluster running the Secret Sync Operator with the credentials to access your remote cluster.
```
apiVersion: v1
kind: Secret
metadata:
  name: secret-sync-remote-cluster-creds
type: Opaque
data:
  # Host for accessing the remote cluster's API
  #   echo -n "https://some.cluster.example.com:443" | base64
  host: REPLACE_WITH_HOST

  # Base64 encoded CA for your remote cluster, you should be able to copy this
  # directly from your ~/.kube/config file
  ca: REPLACE_WITH_CA

  # Access token for the service account that you want to use when syncing secrets.
  # This should be a read-only user that can only access the specific secret(s) you
  # want to sync (eg SSL keys).
  token: REPLACE_WITH_TOKEN
```

Your operator should now have access to read the secrets you want to sync. The last step is creating SynchronizedSecrets to the operator knows which secrets you want to sync.

## Creating a SynchronizedSecret
Finally we create a SynchronizedSecret which finds a secret on the remote cluster matching the remoteSecret spec and creates a local secret with name/namespace matching the metadata on the SynchronizedSecret object.
```
apiVersion: app.gamesight.io/v1alpha1
kind: SynchronizedSecret
metadata:
  name: my-secret-to-distribute
spec:
  remoteSecret:
    name: my-secret-to-distribute
    namespace: default
```
