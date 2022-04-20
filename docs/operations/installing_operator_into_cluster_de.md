# Installationsanleitung für den k8s-service-discovery

## Installation von GitHub

Die Installation von GitHub erfordert die Installations-YAML, die alle benötigten K8s-Ressourcen enthält. In dieser
YAML müssen alle Einträge `{{ .Namespace}}` durch den Ziel-Namespace ersetzt werden.

```bash
# define version
GITHUB_VERSION=0.0.6
TARGET_NAMESPACE=my-namespace

# download yaml
wget https://github.com/cloudogu/k8s-service-discovery/releases/download/v${GITHUB_VERSION}/k8s-dogu-operator_${GITHUB_VERSION}.yaml

# replace namespace and apply service discovery
sed "s/{{ .Namespace }}/${TARGET_NAMESPACE}/" ${K8S_RESOURCE_YAML} | kubectl --namespace "${TARGET_NAMESPACE}" apply -f -
```

Der Operator sollte nun erfolgreich im Cluster gestartet sein.