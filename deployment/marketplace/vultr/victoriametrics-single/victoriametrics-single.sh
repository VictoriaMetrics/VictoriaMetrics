#!/bin/bash
################################################
## Prerequisites
chmod +x /root/vultr-helper.sh
. /root/vultr-helper.sh
error_detect_on
install_cloud_init latest

################################################
## Create victoriametrics user
groupadd -r victoriametrics
useradd -g victoriametrics -d /var/lib/victoria-metrics-data -s /sbin/nologin --system victoriametrics

mkdir -p /var/lib/victoria-metrics-data
chown -R victoriametrics:victoriametrics /var/lib/victoria-metrics-data

################################################
## Download VictoriaMetrics
wget https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/v${VM_VERSION}/victoria-metrics-linux-amd64-v${VM_VERSION}.tar.gz -O /tmp/victoria-metrics.tar.gz
tar xvf /tmp/victoria-metrics.tar.gz -C /usr/bin
chmod +x /usr/bin/victoria-metrics-prod
chown root:root /usr/bin/victoria-metrics-prod

################################################
## Install provisioning scripts
mkdir -p /var/lib/cloud/scripts/per-boot/
mkdir -p /var/lib/cloud/scripts/per-instance/

mv /root/setup-per-boot.sh /var/lib/cloud/scripts/per-boot/setup-per-boot.sh
mv /root/setup-per-instance.sh /var/lib/cloud/scripts/per-instance/setup-per-instance.sh

chmod +x /var/lib/cloud/scripts/per-boot/setup-per-boot.sh
chmod +x /var/lib/cloud/scripts/per-instance/setup-per-instance.sh

# Enable VictoriaMetrics on boot
systemctl enable vmsingle.service

################################################
## Prepare server for Marketplace snapshot

clean_system