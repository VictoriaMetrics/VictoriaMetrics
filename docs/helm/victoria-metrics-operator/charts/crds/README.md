

![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![Version: 0.0.0](https://img.shields.io/badge/Version-0.0.0-informational?style=flat-square)

A subchart stores victoriametrics operator CRDs.

## Documentation of Helm Chart

Install ``helm-docs`` following the instructions on this [tutorial](https://docs.victoriametrics.com/helm/requirements/).

Generate docs with ``helm-docs`` command.

```bash
cd charts/crds

helm-docs
```

The markdown generation is entirely go template driven. The tool parses metadata from charts and generates a number of sub-templates that can be referenced in a template file (by default ``README.md.gotmpl``). If no template file is provided, the tool has a default internal template that will generate a reasonably formatted README.
