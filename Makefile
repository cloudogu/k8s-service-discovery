# Set these to the desired values
ARTIFACT_ID=k8s-service-discovery
VERSION=5.0.1

IMAGE=cloudogu/${ARTIFACT_ID}:${VERSION}
GOTAG?=1.26.0
MAKEFILES_VERSION=10.6.0
LINT_VERSION?=v2.9.0
MOCKERY_VERSION=v2.53.5

ADDITIONAL_CLEAN=dist-clean

STAGE?=production
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
include build/make/release.mk

K8S_COMPONENT_SOURCE_VALUES = ${HELM_SOURCE_DIR}/values.yaml
K8S_COMPONENT_TARGET_VALUES = ${HELM_TARGET_DIR}/values.yaml
PRE_COMPILE=generate-deepcopy
HELM_PRE_GENERATE_TARGETS = helm-values-update-image-version
HELM_POST_GENERATE_TARGETS = helm-values-replace-image-repo template-stage template-log-level template-image-pull-policy
CHECK_VAR_TARGETS=check-all-vars
IMAGE_IMPORT_TARGET=image-import

include build/make/k8s-controller.mk

.PHONY: helm-values-update-image-version
helm-values-update-image-version: $(BINARY_YQ)
	@echo "Updating the image version in source values.yaml to ${VERSION}..."
	@$(BINARY_YQ) -i e ".manager.image.tag = \"${VERSION}\"" ${K8S_COMPONENT_SOURCE_VALUES}

.PHONY: helm-values-replace-image-repo
helm-values-replace-image-repo: $(BINARY_YQ)
	@if [[ ${STAGE} != "production" ]]; then \
      		echo "Setting dev image repo in target values.yaml!" ;\
    		$(BINARY_YQ) -i e ".manager.image.registry=\"$(shell echo '${IMAGE_DEV}' | sed 's/\([^\/]*\)\/\(.*\)/\1/')\"" ${K8S_COMPONENT_TARGET_VALUES} ;\
    		$(BINARY_YQ) -i e ".manager.image.repository=\"$(shell echo '${IMAGE_DEV}' | sed 's/\([^\/]*\)\/\(.*\)/\2/')\"" ${K8S_COMPONENT_TARGET_VALUES} ;\
    	fi

.PHONY: template-stage
template-stage: $(BINARY_YQ)
	@if [[ ${STAGE} != "production" ]]; then \
  		echo "Setting STAGE env in deployment to ${STAGE}!" ;\
		$(BINARY_YQ) -i e ".manager.env.stage=\"${STAGE}\"" ${K8S_COMPONENT_TARGET_VALUES} ;\
	fi

.PHONY: template-log-level
template-log-level: ${BINARY_YQ}
	@if [[ ${STAGE} != "production" ]]; then \
      echo "Setting LOG_LEVEL env in deployment to ${LOG_LEVEL}!" ; \
      $(BINARY_YQ) -i e ".manager.env.logLevel=\"${LOG_LEVEL}\"" "${K8S_COMPONENT_TARGET_VALUES}" ; \
    fi

.PHONY: template-image-pull-policy
template-image-pull-policy: $(BINARY_YQ)
	@if [[ ${STAGE} != "production" ]]; then \
          echo "Setting pull policy to always!" ; \
          $(BINARY_YQ) -i e ".manager.imagePullPolicy=\"Always\"" "${K8S_COMPONENT_TARGET_VALUES}" ; \
    fi

