ARG base_image=non-existing
FROM $base_image

LABEL "org.opencontainers.image.source"="https://github.com/VictoriaMetrics/VictoriaMetrics"

EXPOSE 8429

ENTRYPOINT ["/vmagent-prod"]
ARG src_binary=non-existing
COPY $src_binary ./vmagent-prod
