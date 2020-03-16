ARG base_image
FROM $base_image

EXPOSE 8481

ENTRYPOINT ["/vmselect-prod"]
ARG src_binary
COPY $src_binary ./vmselect-prod
