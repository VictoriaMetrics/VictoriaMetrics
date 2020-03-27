ARG base_image
FROM $base_image

EXPOSE 8428

ENTRYPOINT ["/victoria-metrics-prod"]
ARG src_binary
COPY $src_binary ./victoria-metrics-prod
