---
sort: 2
---

# Additional Scrape Configuration

AdditionalScrapeConfigs is an additional way to add scrape targets in VMAgent CRD.
There are two options for adding targets into VMAgent: inline configuration into CRD or defining it as a Kubernetes Secret.

No validation happens during the creation of configuration. However, you must validate job specs, and it must follow job spec configuration.
Please check official Prometheus documentation as references.
[Prometheus documentation](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config).

## Inline Additional Scrape Configuration in VMAgent CRD

You need to add scrape configuration directly to the vmagent spec.inlineScrapeConfig. It is raw text in YAML format.
See example below

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAgent
metadata:
  name: example-vmagent
spec:
  serviceScrapeSelector: {}
  replicas: 1
  serviceAccountName: vmagent
  inlineScrapeConfig: |
    - job_name: "prometheus"
      static_configs:
      - targets: ["localhost:9090"]
  remoteWrite:
    - url: "http://vmagent-example-vmsingle.default.svc:8429/api/v1/write"
EOF
```

**Note**: Do not use passwords and tokens with inlineScrapeConfig use Secret instead of


## Define Additional Scrape Configuration as a Kubernetes Secret 

You need to define Kubernetes Secret with a key.

The key is `prometheus-additional.yaml` in the example below

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: additional-scrape-configs
stringData:
  prometheus-additional.yaml: |
    - job_name: "prometheus"
      static_configs:
      - targets: ["localhost:9090"]
EOF
```

After that, you need to specify the secret's name and key in VMAgent CRD in `additionalScrapeConfigs` section

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAgent
metadata:
  name: example-vmagent
spec:
  serviceScrapeSelector: {}
  replicas: 1
  serviceAccountName: vmagent
  additionalScrapeConfigs:
    name: additional-scrape-configs
    key: prometheus-additional.yaml
  remoteWrite:
    - url: "http://vmagent-example-vmsingle.default.svc:8429/api/v1/write"
EOF
```

**Note**: You can specify only one Secret in the VMAgent CRD configuration so use it for all additional scrape configurations.

