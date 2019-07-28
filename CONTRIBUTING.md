If you like VictoriaMetrics and want to contribute, then we need the following:

- Filing issues and feature requests [here](https://github.com/VictoriaMetrics/VictoriaMetrics/issues).
- Spreading a word about VictoriaMetrics: conference talks, articles, comments, experience sharing with colleagues.
- Updating documentation.

We are open to third-party pull requests provided they follow [KISS design principle](https://en.wikipedia.org/wiki/KISS_principle):

- Prefer simple code and architecture.
- Avoid complex abstractions.
- Avoid magic code and fancy algorithms.
- Avoid [big external dependencies](https://medium.com/@valyala/stripping-dependency-bloat-in-victoriametrics-docker-image-983fb5912b0d).
- Minimize the number of moving parts in the distributed system.
- Avoid automated decisions, which may hurt cluster availability, consistency or performance.

Adhering `KISS` principle simplifies the resulting code and architecture, so it can be reviewed, understood and verified by many people.
