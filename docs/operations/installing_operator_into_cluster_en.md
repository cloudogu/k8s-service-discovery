# installation instructions for the k8s-service-discovery

## Installing GitHub

Installing GitHub requires the installation YAML, which contains all the required K8s resources. In this
YAML, all entries `{ .namespace}}` must be replaced with the target namespace.

```bash
# define version
GITHUB_VERSION=0.0.6
TARGET_NAMESPACE=my-namespace

# download yaml
wget https://github.com/cloudogu/k8s-service-discovery/releases/download/v${GITHUB_VERSION}/k8s-dogu-operator_${GITHUB_VERSION}.yaml

# replace namespace and apply service discovery
sed "s/{{ .namespace }}/${TARGET_NAMESPACE}/" ${K8S_RESOURCE_YAML} | kubectl --namespace "${TARGET_NAMESPACE}" apply -f -
```

The operator should now be successfully started in the cluster.