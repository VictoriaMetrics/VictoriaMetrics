#!/bin/bash
set -ex

VM_DS_PATH='/var/lib/grafana/plugins/victorialogs-datasource'
PLUGIN_PATH='/var/lib/grafana/plugins'

if [[ -f ${VM_DS_PATH}/plugin.json ]]; then
    ver=$(cat ${VM_DS_PATH}/plugin.json)
    if [[ ! -z "$ver" ]]; then
    exit
    fi
fi

echo "VictoriaLogs datasource is not installed. Installing datasource..."
rm -rf ${VM_DS_PATH}/* || true
mkdir -p ${VM_DS_PATH}

export LATEST_VERSION=$(curl https://api.github.com/repos/VictoriaMetrics/victorialogs-datasource/releases/latest | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' | head -1); \
curl -L https://github.com/VictoriaMetrics/victorialogs-datasource/releases/download/${LATEST_VERSION}/victorialogs-datasource-${LATEST_VERSION}.tar.gz -o ${PLUGIN_PATH}/plugin.tar.gz && \
tar -xzf ${PLUGIN_PATH}/plugin.tar.gz -C ${PLUGIN_PATH}
echo "VictoriaLogs datasource has been installed."
rm ${PLUGIN_PATH}/plugin.tar.gz
