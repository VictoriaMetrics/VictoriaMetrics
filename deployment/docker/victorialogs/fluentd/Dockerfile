FROM fluent/fluentd:v1.17
USER root
RUN \
  gem install \
    fluent-plugin-datadog \
    fluent-plugin-grafana-loki \
    fluent-plugin-elasticsearch \
    fluent-plugin-remote_syslog
USER fluent
