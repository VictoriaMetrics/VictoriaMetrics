RELEASE_NAME := vm-oneclick-droplet
VM_VERSION ?= $(shell git describe --abbrev=0 --tags)

.PHONY: $(MAKECMDGOALS)

release-victoria-metrics-digitalocean-oneclick-droplet:
	cp ./files/etc/update-motd.d/99-one-click.tpl ./files/etc/update-motd.d/99-one-click
	sed -i -e "s/VM_VERSION/${VM_VERSION}/g" ./files/etc/update-motd.d/99-one-click
	packer init template.pkr.hcl
	packer build template.pkr.hcl