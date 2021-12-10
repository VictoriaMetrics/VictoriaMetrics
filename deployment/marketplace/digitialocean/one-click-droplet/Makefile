RELEASE_NAME := vm-oneclick-droplet
VM_VERSION := $(VM_VER)
DIGITALOCEAN_API_TOKEN := $(DIGITALOCEAN_API_TOKEN)

# include deployment/marketplace/digitalocean/one-click-droplet/Makefile

release-victoria-metrics-digitalocean-oneclick-droplet:
	export DIGITALOCEAN_API_TOKEN=$(DIGITALOCEAN_API_TOKEN)
	export VM_VERSION=$(VM_VER)
	packer build template.pkr.hcl