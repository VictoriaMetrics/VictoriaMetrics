---
weight: 300
title: Long-term support releases
menu:
  docs:
    parent: 'victoriametrics'
    weight: 300
tags:
  - metrics
  - enterprise
aliases:
- /LTS-releases.html
- /lts-releases/index.html
- /lts-releases/
---
[Enterprise version of VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/enterprise/) provides long-term support lines of releases (aka LTS releases).
Every LTS line receives bugfixes and [security fixes](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/SECURITY.md) for 12 months after
the initial release. New LTS lines are published every 6 months, so the latest two LTS lines are supported at any given moment. This gives up to 6 months
for the migration to new LTS lines for [VictoriaMetrics Enterprise](https://docs.victoriametrics.com/victoriametrics/enterprise/) users.

LTS releases are published for [Enterprise versions of VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/enterprise/) only.
When a new LTS line is created, the new LTS release might be publicly available for everyone until the new major OS release will be published.

All the bugfixes and security fixes, which are included in LTS releases, are also available in [the latest release](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest),
so non-enterprise users are advised to regularly [upgrade](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-upgrade-victoriametrics) VictoriaMetrics products
to [the latest available releases](https://docs.victoriametrics.com/victoriametrics/changelog/).

## Currently supported LTS release lines

- v1.122.x - the latest one is [v1.122.13 LTS release](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.122.13)
- v1.110.x - the latest one is [v1.110.28 LTS release](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/tag/v1.110.28)
