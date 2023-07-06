# Set these to the desired values
ARTIFACT_ID=k8s-service-discovery
VERSION=0.12.1

## Image URL to use all building/pushing image targets
IMAGE_DEV=${K3CES_REGISTRY_URL_PREFIX}/${ARTIFACT_ID}:${VERSION}
IMAGE=cloudogu/${ARTIFACT_ID}:${VERSION}
GOTAG?=1.20.3
MAKEFILES_VERSION=7.9.0
LINT_VERSION?=v1.52.1

ADDITIONAL_CLEAN=dist-clean

K8S_RESOURCE_DIR=${WORKDIR}/k8s
K8S_WARP_CONFIG_RESOURCE_YAML=${K8S_RESOURCE_DIR}/k8s-ces-warp-config.yaml
K8S_WARP_MENU_JSON_YAML=${K8S_RESOURCE_DIR}/k8s-ces-menu-json.yaml

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
include build/make/mocks.mk

K8S_RUN_PRE_TARGETS=setup-etcd-port-forward
PRE_COMPILE=generate
K8S_PRE_GENERATE_TARGETS=k8s-create-temporary-resource template-dev-only-image-pull-policy

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
	@echo "Add Warp-Config"
	@cp $(K8S_WARP_CONFIG_RESOURCE_YAML) ${K8S_HELM_TARGET}/templates

.PHONY: generate-menu-json
generate-menu-json:
	@echo "Add menu.json"
	@cp $(K8S_WARP_MENU_JSON_YAML) ${K8S_HELM_TARGET}/templates

create-temporary-release-resources: $(K8S_PRE_GENERATE_TARGETS)

.PHONY: template-dev-only-image-pull-policy
template-dev-only-image-pull-policy: $(BINARY_YQ)
	@echo "Setting pull policy to always!"
	@$(BINARY_YQ) -i e "(select(.kind == \"Deployment\").spec.template.spec.containers[]|select(.image == \"*$(ARTIFACT_ID)*\").imagePullPolicy)=\"Always\"" $(K8S_RESOURCE_TEMP_YAML)

##@ Override k8s-helm-generate targets to add menu.json & warp-config
.PHONY: k8s-helm-generate
k8s-helm-generate: k8s-generate ${K8S_HELM_RESSOURCES}/Chart.yaml ${BINARY_HELMIFY} $(K8S_RESOURCE_TEMP_FOLDER) k8s-helm-generate-chart generate-menu-json generate-warp-config ## Generates the final helm chart with dev-urls.

.PHONY: k8s-helm-generate-release
k8s-helm-generate-release: $(K8S_PRE_GENERATE_TARGETS) ${K8S_HELM_RESSOURCES}/Chart.yaml ${BINARY_HELMIFY} $(K8S_RESOURCE_TEMP_FOLDER) k8s-helm-generate-chart generate-menu-json generate-warp-config  ## Generates the final helm chart with release urls.
