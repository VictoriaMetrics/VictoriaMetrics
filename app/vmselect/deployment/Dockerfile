ARG certs_image
FROM $certs_image AS certs
FROM scratch
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ARG src_binary
COPY $src_binary ./vmselect-prod
EXPOSE 8481
ENTRYPOINT ["/vmselect-prod"]
