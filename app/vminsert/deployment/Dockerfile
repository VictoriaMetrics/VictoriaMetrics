FROM scratch
COPY --from=local/certs:1.0.2 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY bin/vminsert-prod .
EXPOSE 8480
ENTRYPOINT ["/vminsert-prod"]
