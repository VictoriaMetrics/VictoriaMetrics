FROM scratch
COPY --from=local/certs:1.0.2 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY bin/victoria-metrics-prod .
EXPOSE 8428
ENTRYPOINT ["/victoria-metrics-prod"]
