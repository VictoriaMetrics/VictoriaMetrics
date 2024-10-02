FROM fluent/fluentd:v1.17-debian-1
USER root
RUN \
  apt update && \
  apt install -y netcat-traditional && \
  gem install \
    fluent-plugin-datadog \
    fluent-plugin-grafana-loki \
    fluent-plugin-elasticsearch \
    fluent-plugin-remote_syslog
USER fluent
