# Set these to the desired values
ARTIFACT_ID=k8s-service-discovery
VERSION=0.1.0

## Image URL to use all building/pushing image targets
IMAGE_DEV=${K3CES_REGISTRY_URL_PREFIX}/${ARTIFACT_ID}:${VERSION}
IMAGE=cloudogu/${ARTIFACT_ID}:${VERSION}
GOTAG?=1.17.7
MAKEFILES_VERSION=6.0.1

ADDITIONAL_CLEAN=dist-clean

K8S_RESOURCE_DIR=${WORKDIR}/k8s
K8S_WARP_CONFIG_RESOURCE_YAML=${K8S_RESOURCE_DIR}/k8s-ces-warp-config.yaml

include build/make/variables.mk
include build/make/self-update.mk
include build/make/dependencies-gomod.mk
include build/make/build.mk
include build/make/test-common.mk
include build/make/test-integration.mk
include build/make/test-unit.mk
include build/make/static-analysis.mk
include build/make/clean.mk
include build/make/digital-signature.mk

K8S_RUN_PRE_TARGETS=setup-etcd-port-forward
PRE_COMPILE=generate vet
K8S_PRE_GENERATE_TARGETS=k8s-create-temporary-resource generate-warp-config

include build/make/k8s-controller.mk


.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	@echo "Generate manifests..."
	@$(CONTROLLER_GEN) rbac:roleName=manager-role webhook paths="./..."

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	@echo "Auto-generate deepcopy functions..."
	@$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

## Local Development

.PHONY: setup-etcd-port-forward
setup-etcd-port-forward:
	kubectl port-forward etcd-0 4001:2379 &

.PHONY: generate-warp-config
generate-warp-config:
	@echo "---" >> $(K8S_RESOURCE_TEMP_YAML)
	@cat $(K8S_WARP_CONFIG_RESOURCE_YAML) >> $(K8S_RESOURCE_TEMP_YAML)


