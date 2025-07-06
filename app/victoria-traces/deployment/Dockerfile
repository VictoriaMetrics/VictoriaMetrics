ARG base_image=non-existing
FROM $base_image

EXPOSE 9428

ENTRYPOINT ["/victoria-traces-prod"]
ARG src_binary=non-existing
COPY $src_binary ./victoria-traces-prod
