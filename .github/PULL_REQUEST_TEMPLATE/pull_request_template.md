### Describe Your Changes

Please provide a brief description of the changes you made. Be as specific as possible to help others understand the purpose and impact of your modifications.

### Checklist (Optional for External Contributions)

- [ ] Include a link to the GitHub issue in the PR description, if possible.
- [ ] Mention the change in the [Changelog](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/docs/CHANGELOG.md). Explain what has changed and why. If there is a related issue or documentation change - link them as well.

  Tips for writing a good changelog message::

    * Write a human-readable changelog message that describes the problem and solution.
    * Include a link to the issue or pull request in your changelog message.
    * Use specific language identifying the fix, such as an error message, metric name, or flag name.
    * Provide a link to the relevant documentation for any new features you add or modify.

- [ ] After your pull request is merged, please add a message to the issue with instructions for how to test the fix or try the feature you added.
- [ ] Do not close the original issue before the change is released. Please note, in some cases Github can automatically close the issue once PR is merged. Re-open the issue in such case.

Examples of good changelog messages:

1. FEATURE: [vmagent](https://docs.victoriametrics.com/vmagent.html): add support for [VictoriaMetrics remote write protocol](https://docs.victoriametrics.com/vmagent.html#victoriametrics-remote-write-protocol) when [sending / receiving data to / from Kafka](https://docs.victoriametrics.com/vmagent.html#kafka-integration). This protocol allows saving egress network bandwidth costs when sending data from `vmagent` to `Kafka` located in another datacenter or availability zone. See [this feature request](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1225).

2. BUGFIX: [stream aggregation](https://docs.victoriametrics.com/stream-aggregation.html): suppress `series after dedup` error message in logs when `-remoteWrite.streamAggr.dedupInterval` command-line flag is set at [vmagent](https://docs.victoriametrics.com/vmgent.html) or when `-streamAggr.dedupInterval` command-line flag is set at [single-node VictoriaMetrics](https://docs.victoriametrics.com/).