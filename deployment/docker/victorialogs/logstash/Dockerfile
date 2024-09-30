FROM docker.elastic.co/logstash/logstash:8.15.0

RUN bin/logstash-plugin install \
    logstash-output-opensearch \
    logstash-output-loki
