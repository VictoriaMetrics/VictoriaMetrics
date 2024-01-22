#!/bin/bash
set -ex

VM_DS_PATH='/var/lib/grafana/plugins/victoriametrics-datasource'
PLUGIN_PATH='/var/lib/grafana/plugins'

if [[ -f ${VM_DS_PATH}/plugin.json ]]; then
    ver=$(cat ${VM_DS_PATH}/plugin.json)
    if [[ ! -z "$ver" ]]; then
    exit
    fi
fi

echo "Victoriametrics datasource is not installed. Installing datasource..."
rm -rf ${VM_DS_PATH}/* || true
mkdir -p ${VM_DS_PATH}

export LATEST_VERSION=$(curl https://api.github.com/repos/VictoriaMetrics/grafana-datasource/releases/latest | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' | head -1); \
curl -L https://github.com/VictoriaMetrics/grafana-datasource/releases/download/${LATEST_VERSION}/victoriametrics-datasource-${LATEST_VERSION}.tar.gz -o ${PLUGIN_PATH}/plugin.tar.gz && \
tar -xzf ${PLUGIN_PATH}/plugin.tar.gz -C ${PLUGIN_PATH}
echo "Victoriametrics datasource has been installed."
rm ${PLUGIN_PATH}/plugin.tar.gz
