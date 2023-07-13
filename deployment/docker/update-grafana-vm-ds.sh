#!/bin/sh
set -ex

VM_DS_PATH='/var/lib/grafana/plugins/victoriametrics-datasource'
PLUGIN_PATH='/var/lib/grafana/plugins'

if [[ -f ${VM_DS_PATH}/plugin.json ]]; then
    ver=$(cat ${VM_DS_PATH}/plugin.json | grep ${VM_DS_VER})
    if [[ ! -z "$ver" ]]; then
    exit
    fi
fi

echo "Victoriametrics datasource is not installed. Installing datasource..."
rm -rf ${VM_DS_PATH}/* || true
mkdir -p ${VM_DS_PATH}
wget -nc https://github.com/VictoriaMetrics/grafana-datasource/releases/download/v${VM_DS_VER}/victoriametrics-datasource-v${VM_DS_VER}.tar.gz -O ${VM_DS_PATH}-v${VM_DS_VER}.tar.gz
tar -xzf ${VM_DS_PATH}-v${VM_DS_VER}.tar.gz -C ${PLUGIN_PATH}
echo "Victoriametrics datasource has been installed."