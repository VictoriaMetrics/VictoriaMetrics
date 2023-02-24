RELEASE_NAME := vm-vultr-server
VM_VERSION ?= $(shell git describe --abbrev=0 --tags)

.PHONY: $(MAKECMDGOALS)

release-victoria-metrics-vultr-server:
	cp ./victoriametrics-single/etc/update-motd.d/99-one-click.tpl ./victoriametrics-single/etc/update-motd.d/99-one-click
	sed -i -e "s/VM_VERSION/${VM_VERSION}/g" ./victoriametrics-single/etc/update-motd.d/99-one-click
	packer init victoriametrics-single/victoriametrics-single.pkr.hcl
	packer build victoriametrics-single/victoriametrics-single.pkr.hcl