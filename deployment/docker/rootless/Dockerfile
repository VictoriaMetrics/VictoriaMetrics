# See https://medium.com/on-docker/use-multi-stage-builds-to-inject-ca-certs-ad1e8f01de1b
FROM alpine:3.10 as rootless
RUN adduser -u 1000 -S -h /home/victoriametrics victoriametrics
USER 1000
