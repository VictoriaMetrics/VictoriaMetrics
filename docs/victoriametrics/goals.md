---
weight: 500
title: Development goals
menu:
  docs:
    parent: 'victoriametrics'
    weight: 500
---
## Goals

1. The main goal - **to help users and [clients](https://docs.victoriametrics.com/enterprise/) resolving issues with VictoriaMetrics components,
   so they could use these components in the most efficient way**.
1. Fixing bugs in the essential functionality of VictoriaMetrics components. Small usability bugs are usually the most annoying,
   so they **must be fixed first**.
1. Improving [docs](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/docs) for VictoriaMetrics components,
   so users could find answers to their questions via Google or [Perplexity](https://www.perplexity.ai/) without the need
   to ask these questions at our [support channels](https://docs.victoriametrics.com/#community-and-contributions).
1. Simplifying usage of VictoriaMetrics components without breaking backwards compatibility, so users could regularly
   upgrade to [the latest available release](https://docs.victoriametrics.com/CHANGELOG) and remain happy.
1. Improving usability for the existing functionality of VictoriaMetrics components.
1. Improving the readability and maintainability of the code base by removing unnecessary abstractions and simplifying the code whenever possible.
1. Improving development velocity by optimizing and simplifying CI/CD tasks, so they take less time to execute and debug.

## Non-goals

1. Convincing people to use VictoriaMetrics components when there are better suited solutions exist for their tasks,
   since users will become angry at VictoriaMetrics after they discover better solutions.
1. Breaking links to [VictoriaMetrics docs](https://docs.victoriametrics.com/), since users will be unhappy seeing 404 page
   or unexpected results after they click some old link somewhere on the Internet or in the internal knowledge base.
1. Breaking backwards compatibility in new releases, since users will be unhappy when their working setup breaks after the upgrade.
1. Adding non-trivial features, which require significant changes in the code and the architecture,
   since these features may break the essential functionality of VictoriaMetrics components, so a big share
   of the existing users may become unhappy after the upgrade.
1. Adding unnecessary abstractions, since they may worsen project maintainability in the future.
1. Implementing all the features users ask. These features should fit [the goals](#goals) of VictoriaMetrics.
   Other feature requests must be closed as `won't implement`, with the link to this page.
1. Merging all the pull requests users submit. These pull requests should fit [the goals](#goals) of VictoriaMetrics.
   Other pull requests must be closed as `won't merge`, with the link to this page.
1. Slowing down and complicating CI/CD pipelines with non-essential tasks, since this results in development velocity slowdown.
1. Introducing non-essential requirements, since this slows down development velocity.

## VictoriaMetrics proverbs

- **Small usability fix is better than non-trivial feature.** Usability fix makes happy existing users.
  Non-trivial feature may make happy some new users, while it may make upset a big share of existing users
  if the feature breaks some essential functionality of VictoriaMetrics components or makes it less efficient.

- **Good docs are better than new shiny feature.** Good docs help users discovering new functionality and dealing
  with VictoriaMetrics components in the most efficient way. Nobody uses new shiny feature if it isn't documented properly.

- **Happy users are more important than the momentary revenue.** Happy users spread the word about VictoriaMetrics,
  so more people convert to VictoriaMetrics users. Happy users are eager to become happy [customers](https://docs.victoriametrics.com/enterprise/).
  This increases long-term revenue.

- **Simple solution is better than smart solution.** Simple solution is easier to setup, operate, debug and troubleshoot than the smart solution.
  This saves users' time, costs and nerve cells.
