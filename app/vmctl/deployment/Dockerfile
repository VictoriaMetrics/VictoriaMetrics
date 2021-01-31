ARG base_image
FROM $base_image

ENTRYPOINT ["/vmctl-prod"]
ARG src_binary
COPY $src_binary ./vmctl-prod
