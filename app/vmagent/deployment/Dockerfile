ARG base_image=non-existing
FROM $base_image

EXPOSE 8429

ENTRYPOINT ["/vmagent-prod"]
ARG src_binary=non-existing
COPY $src_binary ./vmagent-prod
