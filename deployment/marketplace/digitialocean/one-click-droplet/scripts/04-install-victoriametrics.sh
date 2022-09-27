#!/bin/sh

# Wait for cloud-init
cloud-init status --wait

wget https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/${VM_VERSION}/victoria-metrics-linux-amd64-${VM_VERSION}.tar.gz -O /tmp/victoria-metrics.tar.gz
tar xvf /tmp/victoria-metrics.tar.gz -C /usr/bin
chmod +x /usr/bin/victoria-metrics-prod
chown root:root /usr/bin/victoria-metrics-prod


# Enable VictoriaMetrics on boot
systemctl enable vmsingle.service