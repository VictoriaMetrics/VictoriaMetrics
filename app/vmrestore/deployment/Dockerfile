ARG base_image
FROM $base_image

ENTRYPOINT ["/vmrestore-prod"]
ARG src_binary
COPY $src_binary ./vmrestore-prod
