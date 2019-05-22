# All these commands must run from repository root.

HELM_PROJECT=victoria-metrics
HELM_PATH=deployment/k8s/helm/${HELM_PROJECT}
HELM_APP_VERSION=1.0

helm-init:
	@helm init

helm-install:
	helm install $(HELM_PATH) -n $(ENV)

helm-install-dev:
	ENV=dev $(MAKE) helm-install

helm-upgrade:
	helm upgrade $(ENV) $(HELM_PATH)

helm-upgrade-dev:
	ENV=dev $(MAKE) helm-upgrade

helm-delete:
	helm del --purge $(ENV)

helm-delete-dev:
	ENV=dev $(MAKE) helm-delete
