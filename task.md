Implement cardinality estimator.

Place absolute all code in app/cestimator.

It should accept a config via -config in yaml format.
Example of configuration:

```yaml
estimators:
  - stream: foo # required
    filter: 'as promql' #optional
    group: 'label name' # optional
```

For each estimator in config it should create hll counter using https://github.com/axiomhq/hyperloglog lib.
If a group parameter is defined than create a hll counter per group.

The app should accept data in Prometheus remote write protocol. Reuse existing solutions.

expose cardinality on /metrics endpoint in format:

cardinality_estimate{stream="foo",group="label name"} 123