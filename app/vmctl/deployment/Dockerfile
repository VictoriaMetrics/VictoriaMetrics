ARG base_image=non-existing
FROM $base_image

LABEL "org.opencontainers.image.source"="https://github.com/VictoriaMetrics/VictoriaMetrics"

ENTRYPOINT ["/vmctl-prod"]
ARG src_binary=non-existing
COPY $src_binary ./vmctl-prod
