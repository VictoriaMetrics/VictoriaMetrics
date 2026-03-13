VictoriaMetrics Observability Stack integrates with AI assistants through MCP servers and agent skills.
These integrations allow AI agents and automation tools to query metrics, logs, and traces, analyze telemetry data, 
and assist engineers with debugging and observability tasks.

# MCP Servers

MCP (Model Context Protocol) servers expose observability data and operational capabilities to AI assistants in a structured way.
This allows AI agents to query telemetry data, analyze system behavior, and assist engineers in troubleshooting and investigation workflows.

## VictoriaMetrics MCP Server

[VictoriaMetrics MCP Server](https://github.com/VictoriaMetrics/mcp-victoriametrics) provides access to VictoriaMetrics
instances, seamless integration with [VictoriaMetrics APIs](https://docs.victoriametrics.com/victoriametrics/url-examples/) 
and [documentation](https://docs.victoriametrics.com/). 

It offers a comprehensive interface for monitoring, observability, and debugging tasks related to VictoriaMetrics, 
enabling advanced automation and interaction capabilities for engineers and tools.

Capabilities include:
- Query metrics and exploring data (even drawing graphs if your client supports it)
- List and exporting available metrics, labels, labels values and entire time series
- Analyze and testing your alerting and recording rules and alerts
- Show parameters of your VictoriaMetrics instances
- Explore cardinality of your data and metrics usage statistics
- Analyze, trace, prettify and explain your queries
- Debug your relabeling rules, downsampling and retention policy configurations
- Integrate with [VictoriaMetrics Cloud](https://docs.victoriametrics.com/victoriametrics-cloud/)
 
> On YouTube: [How to Use an AI Assistant with Your Monitoring System – VictoriaMetrics MCP Server](https://www.youtube.com/watch?v=1k7xgbRi1k0).

See more details at [VictoriaMetrics/mcp-victoriametrics](https://github.com/VictoriaMetrics/mcp-victoriametrics).

## VictoriaLogs MCP Server

[VictoriaLogs MCP Server](https://github.com/VictoriaMetrics/mcp-victorialogs) provides access to VictoriaLogs instances,
integration with [VictoriaLogs APIs](https://docs.victoriametrics.com/victorialogs/querying/#http-api) and [documentation](https://docs.victoriametrics.com/victorialogs/).

It provides a comprehensive interface for working with logs and performing observability and debugging tasks related to VictoriaLogs.

Capabilities include:
- Querying logs and exploring logs data
- Showing parameters of your VictoriaLogs instances
- Listing available streams, fields, field values
- Query statistics for the logs as metrics

See more details at [VictoriaMetrics/mcp-victorialogs](https://github.com/VictoriaMetrics/mcp-victorialogs).

## VictoriaTraces MCP Server

[VictoriaTraces MCP Server](https://github.com/VictoriaMetrics/mcp-victoriatraces) provides access to VictoriaTraces instances,
integration with [VictoriaTraces APIs](https://docs.victoriametrics.com/victoriatraces/querying/#http-api) and [documentation](https://docs.victoriametrics.com/victoriatraces/).

It enables AI assistants and tools to interact with distributed tracing data for observability and debugging tasks.

Capabilities include:
- Get services and operations (span names)
- Query traces, explore and analyze traces data

See more details at [VictoriaMetrics/mcp-victoriatraces](https://github.com/VictoriaMetrics/mcp-victoriatraces).

## vmanomaly MCP Server

[vmanomaly MCP Server](https://github.com/VictoriaMetrics/mcp-vmanomaly) provides seamless integration with vmanomaly
REST API and documentation for AI-assisted anomaly detection, model management, and observability insights.

Capabilities include:
- Health Monitoring: Check `vmanomaly` server health and build information
- Model Management: List, validate, and configure anomaly detection models (like `zscore_online`, `prophet`, and more)
- Configuration Generation: Generate complete `vmanomaly` YAML configurations
- Alert Rule Generation: Generate [`vmalert`](https://docs.victoriametrics.com/victoriametrics/vmalert/) [alerting rules](https://docs.victoriametrics.com/victoriametrics/vmalert/#alerting-rules) based on [anomaly score metrics](https://docs.victoriametrics.com/anomaly-detection/faq/#what-is-anomaly-score) to simplify alerting setup
- Documentation Search: Full-text search across embedded `vmanomaly` documentation with fuzzy matching

See more details at [VictoriaMetrics/mcp-vmanomaly](https://github.com/VictoriaMetrics/mcp-vmanomaly).


# Agent Skills

[Agent skills](https://github.com/VictoriaMetrics/skills) help AI agents and automation tools understand, operate, 
and troubleshoot VictoriaMetrics observability components, including metrics, logs, and traces.

These skills provide predefined workflows and capabilities such as:
* Query metrics, logs, traces and alerts
* Query trace analysis
* Multi-signal investigations 
* Cardinality optimization 
* Unused metric detection

To install the available skills for AI agents, run:
```sh
npx skills add VictoriaMetrics/skills
```

See more details at [VictoriaMetrics/skills](https://github.com/VictoriaMetrics/skills).