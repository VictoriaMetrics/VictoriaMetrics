FROM scratch
COPY --from=local/certs:1.0.2 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY bin/vmstorage-prod .
EXPOSE 8482
EXPOSE 8400
EXPOSE 8401
ENTRYPOINT ["/vmstorage-prod"]
