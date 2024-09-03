ARG base_image=non-existing
FROM $base_image

EXPOSE 8480

ENTRYPOINT ["/vminsert-prod"]
ARG src_binary=non-existing
COPY $src_binary ./vminsert-prod
