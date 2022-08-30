VERSION ?= $(shell git rev-parse --short HEAD)

PROJECT_DIR ?= $(shell echo $(shell dirname $(shell dirname $(shell pwd))) | xargs)# equal to ../../

## Location to install dependencies to
LOCALBIN ?= $(shell echo $(PROJECT_DIR))/bin
$(LOCALBIN): ## Ensure that the directory exists
	mkdir -p $(LOCALBIN)

## Configuring the environment mode
ENV ?= dev
ifneq ($(origin PROD),undefined)
  ENV = prod
endif

ifeq ($(ENV),prod)
  CONTEXT ?= gke_raptor-test_europe-west3-a_raptor-test
  $(info $(shell tput setaf 1)-+-+ PROD MODE -+-+$(shell tput sgr0))
else
  CONTEXT ?= kind-raptor
  $(info $(shell tput setaf 2)+-+- DEV MODE +-+-$(shell tput sgr0))
endif
KUBECTL = kubectl --context='${CONTEXT}'

$(info $(shell tput setaf 3)PROJ: $(shell tput sgr0)$(PROJECT_DIR)..)
$(info $(shell tput setaf 3)LOCALBIN: $(shell tput sgr0)$(LOCALBIN))
$(info $(shell tput setaf 3)Context: $(shell tput sgr0)$(CONTEXT))
$(info $(shell tput setaf 3)Version: $(shell tput sgr0)$(VERSION))
$(info )

.DEFAULT_GOAL := help

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Setup local cluster

.PHONY: new-local-cluster
new-local-cluster: delete-local-cluster create-local-cluster setup-local-cluster setup-redis rollout-raptor  ## Recreate and setup local cluster with Raptor

.PHONY: create-local-cluster
create-local-cluster: ## Create a Kind cluster
ifeq ($(ENV),prod)
	$(error Cannot create a local cluster in production mode)
else
	kind create cluster --config config/kind-cluster.yaml
	@echo Cluster and node created, waiting for them to be Ready...
	$(KUBECTL) wait --for=condition=Ready nodes/raptor-control-plane --timeout=600s
	$(KUBECTL) config use-context ${CONTEXT}
endif

.PHONY: setup-local-cluster
setup-local-cluster: ## Setup local kind-based cluster with nginx ingress and metrics-server
ifeq ($(ENV),prod)
	$(error Cannot setuo a local cluster in production mode)
else
	$(KUBECTL) apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml && \
	$(KUBECTL) apply -f https://github.com/kubernetes-sigs/metrics-server/releases/download/v0.6.1/components.yaml && \
	$(KUBECTL) patch deployment metrics-server -n kube-system --patch-file ./config/kind-metrics-server-patch.yaml
	@echo Starting wait for ingress-nginx...
	$(KUBECTL) wait --namespace ingress-nginx --for=condition=ready pod \
	  --selector=app.kubernetes.io/component=controller \
	  --timeout=600s
endif

.PHONY: setup-redis
setup-redis: ## Install redis cluster
	-$(KUBECTL) create namespace redis-operator
	$(KUBECTL) apply -f https://raw.githubusercontent.com/OT-CONTAINER-KIT/redis-operator/master/config/crd/bases/redis.redis.opstreelabs.in_redis.yaml
	$(KUBECTL) apply -f https://raw.githubusercontent.com/OT-CONTAINER-KIT/redis-operator/master/config/crd/bases/redis.redis.opstreelabs.in_redisclusters.yaml
	$(KUBECTL) apply -f https://raw.githubusercontent.com/OT-CONTAINER-KIT/redis-operator/master/config/rbac/serviceaccount.yaml
	$(KUBECTL) apply -f https://raw.githubusercontent.com/OT-CONTAINER-KIT/redis-operator/master/config/rbac/role.yaml
	$(KUBECTL) apply -f https://raw.githubusercontent.com/OT-CONTAINER-KIT/redis-operator/master/config/rbac/role_binding.yaml
	$(KUBECTL) apply -f https://raw.githubusercontent.com/OT-CONTAINER-KIT/redis-operator/master/config/manager/manager.yaml
	$(KUBECTL) wait --namespace redis-operator --for=condition=ready pod --selector=control-plane=redis-operator --timeout=120s
	$(KUBECTL) apply -f config/redis.yaml

.PHONY: setup-certmanager
setup-certmanager: ## Setup cert-manager
	$(KUBECTL) apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.8.0/cert-manager.yaml
	$(KUBECTL) wait --namespace cert-manager --for=condition=ready pod --selector=app.kubernetes.io/name=cert-manager --timeout=120s

.PHONY: delete-local-cluster
delete-local-cluster: ## Delete local kind cluster
	@echo "This will delete your current KiND cluster if exists."
	@echo "Are you sure? [y/N] " && read ans && if [ $${ans:-'N'} = 'y' ]; then $(MAKE) _unsafe_delete-cluster ENV=${ENV}; else exit 1; fi

.PHONY: delete-cluster-unsafe
_unsafe_delete-cluster:
	@kind delete cluster --name raptor

##@ Deployment - Raptor

.PHONY: rollout-raptor
rollout-raptor: setup-redis ## build & deploy operator to cluster
	$(MAKE) -C ${PROJECT_DIR} docker-build kind-load ENV=${ENV}
	$(MAKE) deploy-raptor

.PHONY: deploy-raptor
deploy-raptor: setup-redis ## deploy operator to cluster
	$(MAKE) -C ${PROJECT_DIR} deploy ENV=${ENV}
	$(KUBECTL) apply -f config/providers-creds.yaml
	$(MAKE) scale-0
	$(KUBECTL) --namespace raptor-system patch deployment raptor-controller-core --patch-file config/raptor-core-patch.yaml
	$(KUBECTL) --namespace raptor-system patch deployment raptor-historian --patch-file config/raptor-historian-patch.yaml
	$(MAKE) scale-normal

scale-0: remove-webhooks ## Scale down to zero the number of replicas of the operator
	$(KUBECTL) --namespace raptor-system scale deployment raptor-controller-core --replicas 0
	$(KUBECTL) --namespace raptor-system scale deployment raptor-historian --replicas 0
scale-normal: install-webhooks ## Scale back to normal the number of replicas of the operator
	$(KUBECTL) --namespace raptor-system scale deployment raptor-controller-core --replicas 2
	$(KUBECTL) --namespace raptor-system scale deployment raptor-historian --replicas 1

install-samples: ## Install samples
	$(KUBECTL) --namespace default apply -k ${PROJECT_DIR}/config/samples/

remove-webhooks: ## Remove webhooks
	$(KUBECTL) --namespace raptor-system delete validatingwebhookconfiguration raptor-validating-webhook-configuration
	$(KUBECTL) --namespace raptor-system delete mutatingwebhookconfiguration raptor-mutating-webhook-configuration

install-webhooks: ## Install webhooks
	cd ${PROJECT_DIR}/config/webhooks/ && $(KUSTOMIZE) edit set nameprefix raptor-
	$(KUBECTL) --namespace raptor-system apply -k ${PROJECT_DIR}/config/webhooks/
	cd ${PROJECT_DIR}/config/webhooks/ && $(KUSTOMIZE) edit set nameprefix ""

## TOOLS
KUSTOMIZE ?= $(LOCALBIN)/kustomize
