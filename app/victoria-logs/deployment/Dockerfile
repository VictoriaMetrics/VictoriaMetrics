ARG base_image
FROM $base_image

EXPOSE 8428

ENTRYPOINT ["/victoria-logs-prod"]
ARG src_binary
COPY $src_binary ./victoria-logs-prod
