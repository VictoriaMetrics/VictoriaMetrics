#!/bin/bash
## Runs once-and-only-once at first boot per instance.

## Report the start time to a logfile.
echo $(date -u) ": System provisioning started." >> /var/log/per-instance.log

apt update
DEBIAN_FRONTEND=noninteractive apt -qq full-upgrade -y
DEBIAN_FRONTEND=noninteractive apt -qq install -y ufw wget software-properties-common ssh

# Configure UFW

sed -e 's|DEFAULT_FORWARD_POLICY=.*|DEFAULT_FORWARD_POLICY="ACCEPT"|g' \
    -i /etc/default/ufw

ufw allow ssh comment "SSH port"
ufw allow http comment "HTTP port"
ufw allow https comment "HTTPS port"
ufw allow 8428 comment "VictoriaMetrics Single HTTP port"
ufw allow 8089/tcp comment "TCP Influx Listen port for VictoriaMetrics"
ufw allow 8089/udp comment "UDP Influx Listen port for VictoriaMetrics"
ufw allow 2003/tcp comment "TCP Graphite Listen port for VictoriaMetrics"
ufw allow 2003/udp comment "UDP Graphite Listen port for VictoriaMetrics"
ufw allow 4242 comment "OpenTSDB Listen port for VictoriaMetrics"

ufw --force enable

## Report the end time to a logfile.
echo $(date -u) ": System provisioning script is complete." >> /var/log/per-instance.log