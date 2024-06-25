---
sort: 500
weight: 500
title: VictoriaMetrics goals
menu:
  docs:
    parent: 'victoriametrics'
    weight: 500
---

# Goals

VictoriaMetrics project is aimed towards the following goals:

1. **The main goal** - to help customers and users resolving issues with VictoriaMetrics components, so they could use these components
   in the most efficient way.
1. Fixing bugs in the essential functionality of VictoriaMetrics components. Small usability bugs are usually the most annoying,
   so they **must** be fixed first.
1. Improving [docs](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/docs) for VictoriaMetrics components,
   so users could find answers to their questions via Google or [Perplexity](https://www.perplexity.ai/) without the need
   to ask these questions at our [support channels](https://docs.victoriametrics.com/#community-and-contributions).
1. Simplifying usage of VictoriaMetrics components without breaking backwards compatibility, so users could regularly
   upgrade to [the latest available release](https://docs.victoriametrics.com/CHANGELOG.html) and remain happy.
1. Improving **the essential functionality** of VictoriaMetrics components.
1. Improving the readability and maintainability of the code base by removing unnecessary abstractions and simplifying the code whenever possible.
1. Improving development velocity by optimizing CI/CD tasks, so they take less time.

# Non-goals

1. Adding non-trivial features, which require significant changes in the code and the architecture.
   Such features may break the essential functionality of VictoriaMetrics components, so a big share
   of the existing users may become unhappy after the upgrade.
1. Adding unnecessary abstractions, since they may worsen project maintainability in the future.
1. Implementing all the features users ask. These features should fit [the goals](#goals) of VictoriaMetrics. Other features must be closed as `won't implement`.
1. Merging all the pull requests users submit. These pull requests should fit [the goals](#goals) of VictoriaMetrics. Other pull requests must be closed as `won't merge`.
1. Slowing down CI/CD pipelines with non-essential tasks, since this results in development velocity slowdown.
1. Slowing down development velocity with non-essential requirements.
