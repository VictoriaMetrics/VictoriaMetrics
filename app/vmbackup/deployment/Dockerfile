ARG base_image
FROM $base_image

ENTRYPOINT ["/vmbackup-prod"]
ARG src_binary
COPY $src_binary ./vmbackup-prod
