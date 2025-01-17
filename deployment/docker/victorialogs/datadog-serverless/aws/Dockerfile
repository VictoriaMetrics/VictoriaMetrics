FROM golang:1.23-bullseye as aws-lambda-rie

# Install custom aws-lambda-rie till Telemetry API support is not merged
# https://github.com/aws/aws-lambda-runtime-interface-emulator/pull/137

RUN \
  git clone https://github.com/VictoriaMetrics/aws-lambda-runtime-interface-emulator -b added-telemetry-api-support /tmp/aws-lambda-rie && \
  cd /tmp/aws-lambda-rie && \
  CGO_ENABLED=0 go build -buildvcs=false -ldflags "-s -w" -o /aws-lambda-rie ./cmd/aws-lambda-rie

FROM python:3.12-bullseye

RUN \
  apt update && \
  apt install -y \
    curl \
    g++ \
    make \
    cmake \
    unzip \
    libcurl4-openssl-dev && \
  mkdir -p /var/task && \
  pip install \
    --target /var/task awslambdaric datadog-lambda

WORKDIR /var/task
COPY --from=aws-lambda-rie /aws-lambda-rie /var/task/aws-lambda-rie
COPY main.py /var/task/
COPY --from=public.ecr.aws/datadog/lambda-extension:68 /opt/. /opt/

ENTRYPOINT ["/var/task/aws-lambda-rie"]
CMD ["/usr/local/bin/python", "-m", "awslambdaric", "main.lambda_handler"]
