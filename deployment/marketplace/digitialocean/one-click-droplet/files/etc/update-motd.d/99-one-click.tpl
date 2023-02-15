#!/bin/sh
#
# Configured as part of the DigitalOcean 1-Click Image build process

myip=$(hostname -I | awk '{print$1}')
cat <<EOF
********************************************************************************

Welcome to DigitalOcean's 1-Click VictoriaMetrics Droplet.
To keep this Droplet secure, the UFW firewall is enabled.
All ports are BLOCKED except 22 (SSH), 80 (HTTP), and 443 (HTTPS), 8428 (VictoriaMetrics HTTP), 8089 (VictoriaMetrics Influx),
4242 (VictoriaMetrics OpenTSDB), 2003 (VictoriaMetrics Graphite)

In a web browser, you can view:
 * The VictoriaMetrics 1-Click Quickstart guide: https://kutt.it/1click-quickstart

On the server:
  * The default VictoriaMetrics root is located at /var/lib/victoria-metrics-data
  * VictoriaMetrics is running on ports: 8428, 8089, 4242, 2003 and they are bound to the local interface.

********************************************************************************
  # This image includes VM_VERSION version of VictoriaMetrics. 
  # See Release notes https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/vVM_VERSION

  # Welcome to VictoriaMetrics droplet!

  # Website:       https://victoriametrics.com
  # Documentation: https://docs.victoriametrics.com
  # VictoriaMetrics Github : https://github.com/VictoriaMetrics/VictoriaMetrics
  # VictoriaMetrics Slack Community: https://slack.victoriametrics.com
  # VictoriaMetrics Telegram Community: https://t.me/VictoriaMetrics_en

  # VictoriaMetrics config:   /etc/victoriametrics/single/victoriametrics.conf

********************************************************************************
EOF
