FROM python:3.12-bullseye

COPY --from=datadog/serverless-init:1 /datadog-init /app/datadog-init
ENV DD_SERVICE=datadog-demo-run-go
ENV DD_ENV=datadog-demo
ENV DD_VERSION=1

RUN pip install Flask gunicorn datadog

WORKDIR /var/task
COPY main.py /var/task/

ENTRYPOINT ["/app/datadog-init"]
CMD exec gunicorn --bind :$PORT --workers 1 --threads 8 --timeout 0 main:app
