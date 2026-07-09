---
weight: 501
title: Release process guidance
menu:
  docs:
    parent: 'victoriametrics'
    identifier: "victoriametrics-release-process-guidance"
    weight: 501
tags: []
aliases:
- /Release-Guide.html
- /release-guide/index.html
- /release-guide/
---

> To maintainers:
>
> The detailed guidance document has been moved to [VictoriaMetrics/release](https://github.com/VictoriaMetrics/release/blob/main/README.md). 
> The new document contains up-to-date release process guidance. 
> Please refer to it instead.
>
> An archived version of this document is available at: [Release-Guide.md](https://github.com/VictoriaMetrics/release/blob/main/legacy_docs/Release-Guide.md).


## Release Process Overview

VictoriaMetrics releases follow a two-step process: a release candidate phase and a final release.
Both the latest and LTS releases are typically done on a bi-weekly cadence.

### Step 1 — Release Candidate (usually Friday)

A release candidate (RC) is prepared and published to VM sandbox for testing.

Key steps:

* Verify all branches are in sync.
* Ensure relevant bug fixes are backported where needed (e.g., LTS versions).
* Run tests and basic validation.
* Check for known vulnerabilities (dependencies and base images).
* Build release binaries and Docker images.
* Publish **release candidate images** (with `-rc` suffix).
* Create a **draft GitHub release** (not published yet).
* Deploy the release candidate to a sandbox/testing environment.

The goal of this phase is to validate the release in a real environment before making it official.

### Step 2 — Final Release (usually Monday)

If the release candidate performs well in testing, it is promoted to a final release.

Key steps:

* Review stability and performance of the RC in the sandbox.
* Publish final Docker images (without `-rc` suffix) and update `latest` tag.
* Perform a quick smoke test on final images.
* Publish the GitHub release.
* Close issues included in the release.
* Update version references in documentation and related projects (operator, helm-charts, ansible-playbook).

