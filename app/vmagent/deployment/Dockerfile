ARG base_image
FROM $base_image

EXPOSE 8429

ENTRYPOINT ["/vmagent-prod"]
ARG src_binary
COPY $src_binary ./vmagent-prod
