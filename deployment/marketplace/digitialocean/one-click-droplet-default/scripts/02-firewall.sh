#!/bin/sh

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