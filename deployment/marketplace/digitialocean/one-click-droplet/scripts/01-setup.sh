#!/bin/sh

# Create victoriametrics user
groupadd -r victoriametrics
useradd -g victoriametrics -d /var/lib/victoria-metrics-data -s /sbin/nologin --system victoriametrics

mkdir -p /var/lib/victoria-metrics-data
chown -R victoriametrics:victoriametrics /var/lib/victoria-metrics-data

rm -rf /var/lib/apt/lists/*
apt update
DEBIAN_FRONTEND=noninteractive apt -y full-upgrade
DEBIAN_FRONTEND=noninteractive apt -y install curl git wget software-properties-common
rm -rf /var/log/kern.log
rm -rf /var/log/ufw.log