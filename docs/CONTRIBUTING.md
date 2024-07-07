---
sort: 400
weight: 400
title: Contributing to VictoriaMetrics
menu:
  docs:
    parent: 'victoriametrics'
    weight: 400
aliases:
- /CONTRIBUTING.html
---

# Contributing to VictoriaMetrics

If you like VictoriaMetrics and want to contribute, then it would be great:

- Joining VictoriaMetrics community Slack ([Slack inviter](https://slack.victoriametrics.com/) and [Slack channel](https://victoriametrics.slack.com/))
  and helping other community members there.
- Filing issues, feature requests and questions [at VictoriaMetrics GitHub](https://github.com/VictoriaMetrics/VictoriaMetrics/issues).
- Improving [VictoriaMetrics docs](https://docs.victoriametrics.com/). See how to update docs [here](https://docs.victoriametrics.com/#documentation).
- Spreading the word about VictoriaMetrics via various channels:
  - conference talks
  - blogposts, articles and [case studies](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/docs/CaseStudies.md)
  - comments at Hacker News, Twitter, LinkedIn, Reddit, Facebook, etc.
  - experience sharing with colleagues.
- Convincing your management to sign [Enterprise contract](https://docs.victoriametrics.com/enterprise/) with VictoriaMetrics.

## Pull request checklist

Before sending a pull request to [VictoriaMetrics repository](https://github.com/VictoriaMetrics/VictoriaMetrics/) please make sure it **conforms all** the following checks:

- The pull request conforms [VictoriaMetrics goals](https://docs.victoriametrics.com/goals/).
- The pull request conforms [`KISS` principle](https://en.wikipedia.org/wiki/KISS_principle). See [these docs](#kiss-principle) for more details.
- The pull request contains clear description of the change, with links to the related GitHub issues and [docs](https://docs.victoriametrics.com/), if needed.
- Commit messages contain concise yet clear descriptions. Include links to related GitHub issues in commit messages, if such issues exist.
- All the commits are signed and include `Signed-off-by` line. Use `git commit -s` to include `Signed-off-by` your commits.
  See [this doc](https://git-scm.com/book/en/v2/Git-Tools-Signing-Your-Work) about how to sign git commits.
- All the lint checks are passing locally via `make check-all` command run from the VictoriaMetrics repository root.
- All the tests are passing locally via `make test-full` command run from the VictoriaMetrics repository root.
- If the change fixes some bug, it would be great to cover it by [tests](https://pkg.go.dev/testing) if it isn't covered yet by existsing tests.
- If the change improves performance or reduces resource usage, then it would be great to add [benchmarks](https://pkg.go.dev/testing#hdr-Benchmarks)
  and mention benchmark results before and after the change in the description to the pull request.
- If the change implements some specifications or uses some external APIs, then please provide permanent links to these specs and APIs
  directly in the relevant source code, in order to simplify further maintenance of the code.
- If the change modifies the existing logic, make sure it doesn't break existing user setups after the upgrade.
- Please investigate git commit history for the code you change in order to make sure your change doesn't break historical conventions in the modified code.

Further checks are optional for external contributions:

- The change must be described in **clear user-readable** form at [docs/CHANGELOG.md](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/docs/CHANGELOG.md),
  since it is read by **VictoriaMetrics users** who may not know implementation details of VictoriaMetrics products. The change description must **clearly** answer the following questions:
  - What does this change do?
  - Why this change is needed?

  The change description must link to the related GitHub issues and the related docs, if any.

  Tips for writing a good changelog message:

  - Write a human-readable changelog message that describes the problem and the solution.
  - Use specific text, which can be googled by users interested in the change, such as an error message, metric name, command-line flag name, etc.
  - Provide a link to the related GitHub issue or pull request.
  - Provide a link to the relevant documentation if the change modifies user-visible behaviour of VictoriaMetrics producs.

- After your pull request is merged, please add a message to the issue with instructions for how to test the change you added before the new release.
  [Here is an example](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4048#issuecomment-1546453726).
- Do not close the original issue before the change is released. In some cases Github can automatically close the issue once PR is merged. Re-open the issue in such case.
- If the change introduces a new feature, this feature must be documented in **user-readable** form at the appropriate parts of [VictoriaMetrics docs](https://docs.victoriametrics.com/).
  The docs' sources are located in the [`docs` folder](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/docs).

Examples of good changelog messages:

* FEATURE: [vmagent](https://docs.victoriametrics.com/vmagent/): add support for [VictoriaMetrics remote write protocol](https://docs.victoriametrics.com/vmagent/#victoriametrics-remote-write-protocol) when [sending / receiving data to / from Kafka](https://docs.victoriametrics.com/vmagent/#kafka-integration). This protocol allows saving egress network bandwidth costs when sending data from `vmagent` to `Kafka` located in another datacenter or availability zone. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1225).

* BUGFIX: [stream aggregation](https://docs.victoriametrics.com/stream-aggregation/): suppress `series after dedup` error message in logs when `-remoteWrite.streamAggr.dedupInterval` command-line flag is set at [vmagent](https://docs.victoriametrics.com/vmgent.html) or when `-streamAggr.dedupInterval` command-line flag is set at [single-node VictoriaMetrics](https://docs.victoriametrics.com/).

## KISS principle

We are open to third-party pull requests provided they follow [KISS design principle](https://en.wikipedia.org/wiki/KISS_principle).

- Prefer simple code and architecture.
- Avoid complex abstractions.
- Avoid magic code and fancy algorithms.
- Apply optimizations, which make the code harder to understand, only if [profiling](https://docs.victoriametrics.com/#profiling)
  shows significant improvements in performance and scalability or significant reduction in RAM usage.
  Profiling must be performed on [Go benchmarks](https://pkg.go.dev/testing#hdr-Benchmarks) and on production workload.
- Avoid [big external dependencies](https://medium.com/@valyala/stripping-dependency-bloat-in-victoriametrics-docker-image-983fb5912b0d).
- Minimize the number of moving parts in the distributed system.
- Avoid automated decisions, which may hurt cluster availability, consistency, performance or debuggability.

Adhering `KISS` principle simplifies the resulting code and architecture, so it can be reviewed, understood and debugged by wider audience.

Due to `KISS`, [cluster version of VictoriaMetrics](https://docs.victoriametrics.com/cluster-victoriametrics/) has no the following "features" popular in distributed computing world:

- Fragile gossip protocols. See [failed attempt in Thanos](https://github.com/improbable-eng/thanos/blob/030bc345c12c446962225221795f4973848caab5/docs/proposals/completed/201809_gossip-removal.md).
- Hard-to-understand-and-implement-properly [Paxos protocols](https://www.quora.com/In-distributed-systems-what-is-a-simple-explanation-of-the-Paxos-algorithm).
- Complex replication schemes, which may go nuts in unforeseen edge cases. See [replication docs](https://docs.victoriametrics.com/cluster-victoriametrics/#replication-and-data-safety) for details.
- Automatic data reshuffling between storage nodes, which may hurt cluster performance and availability.
- Automatic cluster resizing, which may cost you a lot of money if improperly configured.
- Automatic discovering and addition of new nodes in the cluster, which may mix data between dev and prod clusters :)
- Automatic leader election, which may result in split brain disaster on network errors.
