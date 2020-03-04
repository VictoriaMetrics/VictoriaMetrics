ARG certs_image
ARG rootless_image

FROM $certs_image AS certs
FROM $rootless_image AS rootless
FROM scratch

COPY --from=rootless --chown=0:0 /etc/passwd /etc/passwd
COPY --from=rootless --chown=0:0 /etc/group /etc/group
USER 1000
COPY --from=rootless --chown=1000 /home/victoriametrics /home/victoriametrics
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

ARG src_binary
COPY $src_binary ./vmstorage-prod

EXPOSE 8482
EXPOSE 8400
EXPOSE 8401

ENTRYPOINT ["/vmstorage-prod", "--storageDataPath=/home/victoriametrics/vmstorage-data"]
