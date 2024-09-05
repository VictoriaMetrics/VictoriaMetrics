ARG base_image=non-existing
FROM $base_image

EXPOSE 8427

ENTRYPOINT ["/vmauth-prod"]
ARG src_binary=non-existing
COPY $src_binary ./vmauth-prod
