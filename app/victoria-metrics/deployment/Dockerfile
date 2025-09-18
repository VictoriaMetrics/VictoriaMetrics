ARG base_image=non-existing
FROM $base_image

EXPOSE 8428

ENTRYPOINT ["/victoria-metrics-prod"]
ARG src_binary=non-existing
COPY $src_binary ./victoria-metrics-prod
