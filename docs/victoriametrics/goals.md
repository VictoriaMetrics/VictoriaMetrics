---
weight: 500
title: Development Goals
menu:
  docs:
    weight: 500
    identifier: goals
    pageRef: "/victoriametrics/goals/"
tags: []
aliases:
  - /goals/index.html
  - /goals/
---
## Goals

1. The main goal - **to help users and [clients](https://docs.victoriametrics.com/victoriametrics/enterprise/) using VictoriaMetrics products in the most efficient way**.
1. Fixing bugs in the essential functionality of VictoriaMetrics components. Small usability bugs are usually the most annoying,
   so they **must be fixed first**. Bugs, which affect a small number of users at some rare edge cases, can be fixed later.
1. Improving [public docs for VictoriaMetrics products](https://docs.victoriametrics.com),
   so users could find answers to their questions via Google or any other AI-powered web search without the need
   to ask these questions at our [support channels](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#community-and-contributions).
1. Simplifying usage of VictoriaMetrics products without breaking backwards compatibility, so users could regularly
   upgrade to [the latest available release](https://docs.victoriametrics.com/victoriametrics/changelog/) and become happier.
1. Improving the usability for the existing functionality of VictoriaMetrics components.
1. Improving the readability and maintainability of the code base by removing unnecessary abstractions and simplifying the code whenever possible.
1. Improving development velocity by optimizing and simplifying CI/CD tasks, so they take less time to execute and debug.

## Non-goals

1. Convincing people to use VictoriaMetrics products when there are better suited solutions exist for their tasks,
   since users will become angry at VictoriaMetrics after they discover better solutions.
1. Breaking links to [VictoriaMetrics docs](https://docs.victoriametrics.com/victoriametrics/), since users will be unhappy seeing 404 page
   or unexpected results after they click some old link somewhere on the Internet or in the internal knowledge base.
1. Breaking backwards compatibility in new releases, since users will be unhappy when their working setup breaks after the upgrade.
1. Adding non-trivial features, which require significant changes in the code and the architecture,
   since these features may break the essential functionality of VictoriaMetrics components, so a big share
   of the existing users may become unhappy after the upgrade.
1. Adding unnecessary abstractions, since they may worsen project maintainability in the future.
1. Implementing all the features users ask. These features should fit [the goals](https://docs.victoriametrics.com/victoriametrics/goals/#goals) of VictoriaMetrics.
   Other feature requests must be closed as `won't implement`, with the link to this page.
1. Merging all the pull requests users submit. These pull requests should fit [the goals](https://docs.victoriametrics.com/victoriametrics/goals/#goals) of VictoriaMetrics.
   Other pull requests must be closed as `won't merge`, with the link to this page.
1. Slowing down and complicating CI/CD pipelines with non-essential tasks, since this results in development velocity slowdown.
1. Introducing non-essential requirements, since this slows down development velocity.

## VictoriaMetrics proverbs

- **A small usability improvement is more valuable than a major new feature.**
  The usability fix makes happy existing users. Non-trivial feature may make happy some new users,
  while it may make upset a big share of existing users if the feature breaks the essential functionality
  of VictoriaMetrics products or makes it less efficient.

- **Having clean and concise documentation is more valuable than having a lot of great but undocumented features.**
  Good docs help users discovering new functionality and dealing with VictoriaMetrics products in the most efficient way.
  Nobody uses new shiny features if they aren't documented properly.

- **A simple solution is better than a smart one.**
  The simple solution is easier to setup, operate, debug and troubleshoot than the smart solution. This saves users' time, costs and nerve cells.

- **Happy users are more important than the short-term profit.**
  Happy users spread the word about VictoriaMetrics, so more people convert to VictoriaMetrics users.
  Happy users are eager to become happy [customers](https://docs.victoriametrics.com/victoriametrics/enterprise/)
  over time. This increases long-term profit.
  Upset users may be forced to become customers, but they will constantily search for the opportunity to switch to competing solutions.
