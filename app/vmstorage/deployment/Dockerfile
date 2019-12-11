ARG certs_image
FROM $certs_image AS certs
FROM scratch
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ARG src_binary
COPY $src_binary ./vmstorage-prod
EXPOSE 8482
EXPOSE 8400
EXPOSE 8401
ENTRYPOINT ["/vmstorage-prod"]
