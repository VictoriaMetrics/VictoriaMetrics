ARG certs_image
FROM $certs_image AS certs
FROM scratch
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ARG src_binary
COPY $src_binary ./vminsert-prod
EXPOSE 8480
ENTRYPOINT ["/vminsert-prod"]
