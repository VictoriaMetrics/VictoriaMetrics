---
weight: 400
title: Contributing
menu:
  docs:
    identifier: vm-contributing
    parent: victoriametrics
    weight: 400
tags: []
aliases:
- /CONTRIBUTING.html
- /contributing/index.html
- /contributing/
---
If you like VictoriaMetrics and want to contribute, then it would be great:

- Joining VictoriaMetrics community Slack ([Slack inviter](https://slack.victoriametrics.com/) and [Slack channel](https://victoriametrics.slack.com/))
  and helping other community members there.
- Filing issues, feature requests and questions [at VictoriaMetrics GitHub](https://github.com/VictoriaMetrics/VictoriaMetrics/issues).
- Improving [VictoriaMetrics docs](https://docs.victoriametrics.com/victoriametrics/). See how to update our [documentation](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#documentation).
- Spreading the word about VictoriaMetrics via various channels:
  - conference talks
  - blogposts, articles and [case studies](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/docs/victoriametrics/CaseStudies.md)
  - comments at Hacker News, Twitter, LinkedIn, Reddit, Facebook, etc.
  - experience sharing with colleagues.
- Convincing your management to sign [Enterprise contract](https://docs.victoriametrics.com/victoriametrics/enterprise/) with VictoriaMetrics.

## Issues

When making a new issue, make sure to create no duplicates. Use GitHub search to find whether similar issues exist already.
The new issue should be written in English and contain concise description of the problem and environment where it exists.
We'd very much prefer to have a specific use-case included in the description, since it could have workaround or alternative solutions.

When looking for an issue to contribute, always prefer working on [bugs](https://github.com/VictoriaMetrics/VictoriaMetrics/issues?q=is%3Aopen+is%3Aissue+label%3Abug)
instead of [enhancements](https://github.com/VictoriaMetrics/VictoriaMetrics/issues?q=is%3Aopen+is%3Aissue+label%3Aenhancement).
Helping other people with their [questions](https://github.com/VictoriaMetrics/VictoriaMetrics/issues?q=is%3Aopen+is%3Aissue+label%3Aquestion) is also a contribution.

If you would like to contribute to [documentation](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/docs), please
read the [guideline](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#documentation).

### Labels

We use [labels](https://docs.github.com/en/issues/using-labels-and-milestones-to-track-work/managing-labels) to categorize GitHub issues. We have the following labels:

1. A component label: vmalert, vmagent, etc. Add this label to the issue if it is related to a specific component.
1. An issue type: `bug`, `enhancement`, `question`.
1. `enterprise`, assigned to issues related to ENT features
1. `need more info`, assigned to issues that require elaboration from the issue creator.
  For example, if we weren't able to reproduce the reported bug based on the ticket description then we ask additional
  questions which could help to reproduce the issue and add `need more info` label. This label helps other maintainers
  to understand that this issue wasn't forgotten but waits for the feedback from user.
1. `waiting for release`, assigned to issues that required code changes and those changes were merged to upstream, but not released yet.
  Once a release is made, maintainers go through all labeled issues, leave a comment about the new release, remove the label, and close the issue.
1. `vmui`, assigned to issues related to [vmui](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#vmui) or [VictoriaLogs webui](https://docs.victoriametrics.com/victorialogs/querying/#web-ui)

## Pull Request checklist

Implementing a bugfix or enhancement requires sending a pull request to the [corresponding repository](https://github.com/orgs/VictoriaMetrics/repositories).

Pull requests requirements:

1. The pull request must conform to [VictoriaMetrics development goals](https://docs.victoriametrics.com/victoriametrics/goals/).
1. Don't use `master` branch for making PRs, as it makes it impossible for reviewers to modify the changes.
1. All commits need to be [signed](https://docs.github.com/en/authentication/managing-commit-signature-verification/signing-commits).
1. A commit message should contain clear and concise description of what was done and for what purpose.
   Use the imperative, present tense: "change" not "changed" nor "changes". Read your commit message as "This commit will ..", don't capitalize the first letter.
   Message should be prefixed with `<dir>/<component>:` to show what component has been changed, i.e. `app/vmalert: fix...`.
1. A link to the issue(s) related to the change, if any. Use `Fixes [issue link]` if the PR resolves the issue, or `Related to [issue link]` for reference.
1. Tests proving that the change is effective. See [this style guide](https://itnext.io/f-tests-as-a-replacement-for-table-driven-tests-in-go-8814a8b19e9e) for tests.
   To run tests and code checks locally, execute commands `make test-full` and `make check-all`.
1. Try to not extend the scope of the pull requests outside the issue, do not make unrelated changes.
1. Update [docs](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/docs) if needed. For example, adding a new flag or changing behavior of existing flags or features
   requires reflecting these changes in the documentation. For new features add `{{%/* available_from "#" */%}}` shortcode to the documentation.
   It will be later automatically replaced with an actual release version.
1. A line in the [changelog](https://docs.victoriametrics.com/victoriametrics/changelog/#tip) mentioning the change and related issue in a way
   that would be clear to other readers even if they don't have the full context.
1. Avoid modifying code in the `/vendor` folder manually, even when the vendored package originates are from the VictoriaMetrics GitHub organization.
   For instance, VictoriaLogs vendors packages under the `/lib` folder from VictoriaMetrics, and VictoriaTraces vendors the `/lib/logstorage` package from VictoriaLogs.
   Submit a pull request to the upstream repository first. Afterward, a separate pull request can be opened to update the version of the vendored folder in downstream repository.
   The update of vendored package can be done with: run `go get` with the **tag** (avoid using the commit hash),
   and then run `go mod tidy` and `go mod vendor` to update the `go.mod`, `go.sum` and `/vendor`.
1. Ping reviewers who you think have the best expertise on the matter.

See good example of a [pull request](https://github.com/VictoriaMetrics/VictoriaMetrics/pull/6487).

## Merging Pull Request

The person who merges the Pull Request is responsible for satisfying requirements below:

1. Make sure that PR satisfies [Pull Request checklist](https://docs.victoriametrics.com/victoriametrics/contributing/#pull-request-checklist),
   it is approved by at least one reviewer, all CI checks are green.
1. Try doing your best at assessing the changes. If possible, test them locally.
1. Once merged, make sure to cherry-pick the changes across all related branches.
1. If applicable, cherry-pick the change to [LTS release lines](https://docs.victoriametrics.com/victoriametrics/lts-releases/)
   and mention in the PR comment what was or wasn't cherry-picked.
1. Update related issues with a meaningful message of what has changed and when it will be
   released. _This helps users to understand the change without reading PR._
1. Add label `waiting for release` to related issues.
1. Do not close related tickets until release is made. If ticket was auto-closed by GitHub or user - re-open it.

## KISS principle

We are open to third-party pull requests provided they follow [KISS design principle](https://en.wikipedia.org/wiki/KISS_principle).

- Prefer simple code and architecture.
- Avoid complex abstractions.
- Avoid magic code and fancy algorithms.
- Apply optimizations, which make the code harder to understand only if [profiling](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#profiling)
  shows significant improvements in performance and scalability or significant reduction in RAM usage.
  Profiling must be performed on [Go benchmarks](https://pkg.go.dev/testing#hdr-Benchmarks) and on production workload.
- Avoid [big external dependencies](https://medium.com/@valyala/stripping-dependency-bloat-in-victoriametrics-docker-image-983fb5912b0d).
- Minimize the number of moving parts in the distributed system.
- Avoid automated decisions, which may hurt cluster availability, consistency, performance or debuggability.

Adhering to `KISS` principle, simplifies the resulting code and architecture so it can be reviewed, understood and debugged by a wider audience.

Due to `KISS`, [cluster version of VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/) has none of the following "features" popular in distributed computing world:

- Fragile gossip protocols. See [failed attempt in Thanos](https://github.com/improbable-eng/thanos/blob/030bc345c12c446962225221795f4973848caab5/docs/proposals/completed/201809_gossip-removal.md).
- Hard-to-understand-and-implement-properly [Paxos protocols](https://www.quora.com/In-distributed-systems-what-is-a-simple-explanation-of-the-Paxos-algorithm).
- Complex replication schemes, which may go nuts in unforeseen edge cases. See [replication docs](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#replication-and-data-safety) for details.
- Automatic data reshuffling between storage nodes, which may hurt cluster performance and availability.
- Automatic cluster resizing, which may cost you a lot of money if improperly configured.
- Automatic discovering and addition of new nodes in the cluster, which may mix data between dev and prod clusters :)
- Automatic leader election, which may result in split brain disaster on network errors.
