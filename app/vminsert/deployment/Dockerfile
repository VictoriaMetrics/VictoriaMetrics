ARG base_image
FROM $base_image

EXPOSE 8480

ENTRYPOINT ["/vminsert-prod"]
ARG src_binary
COPY $src_binary ./vminsert-prod
