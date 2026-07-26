VictoriaMetrics Observability Stack integrates with AI assistants through [MCP servers](https://docs.victoriametrics.com/ai-tools/#mcp-servers)
and [agent skills](https://docs.victoriametrics.com/ai-tools/#agent-skills).
The integrations allow AI agents and automation tools to query Metrics, Logs, and Traces, analyze telemetry data, 
and assist engineers with debugging, observability tasks, root cause analysis, anomaly detection, etc.

Support of [OpenTelemetry](https://docs.victoriametrics.com/opentelemetry/) for Metrics, Logs, and Traces
makes VictoriaMetrics Observability Stack optimal for [AI observability](https://docs.victoriametrics.com/ai-tools/#ai-observability).
Any SDK or AI assistant that can emit telemetry signals in OpenTelemetry format can be integrated with VictoriaMetrics.

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
- Check `vmanomaly` health, build information, compatibility, and self-monitoring metrics
- Inspect model schemas and validate model or complete service configurations
- Profile sampled query results for trends, seasonalities, changepoints, gaps, and intermittent behavior
- Run asynchronous autotune tasks and turn their results into data-driven model recommendations
- Generate complete `vmanomaly` YAML configurations and [`vmalert`](https://docs.victoriametrics.com/victoriametrics/vmalert/) [alerting rules](https://docs.victoriametrics.com/victoriametrics/vmalert/#alerting-rules)
- Search embedded `vmanomaly` documentation with fuzzy matching

See more details at [VictoriaMetrics/mcp-vmanomaly](https://github.com/VictoriaMetrics/mcp-vmanomaly).
# vmanomaly UI Copilot
The vmanomaly UI includes an [AI Copilot](https://docs.victoriametrics.com/anomaly-detection/ui/#ai-assistance) that can assist users with anomaly detection tasks, model configuration, and troubleshooting, changing the UI state based on user queries and providing actionable suggestions through automated data profiling and validation. The AI Copilot is powered by respective [MCP Server](#vmanomaly-mcp-server) and [Agent Skills](#agent-skills), enabling it to understand the context of the user's actions and provide relevant guidance.

# Agent Skills

[Agent skills](https://github.com/VictoriaMetrics/skills) help AI agents and automation tools understand, operate,
and troubleshoot VictoriaMetrics observability components, including metrics, logs, traces, and [`vmanomaly`](https://docs.victoriametrics.com/anomaly-detection/).

These skills provide predefined workflows and capabilities such as:
* Query metrics, logs, traces and alerts
* Query trace analysis
* Multi-signal investigations 
* Cardinality optimization 
* Unused metric detection
* Stream aggregation configuration
* Build validated `vmanomaly` configurations from measured time-series characteristics (e.g., seasonality, changepoints, trends)
* Query and operate the `vmanomaly` API
* Review existing anomaly detection configurations against real data and identify false-positive or model-data fit issues

To install the available skills for AI agents, run:
```sh
npx skills add VictoriaMetrics/skills
```

See more details at [VictoriaMetrics/skills](https://github.com/VictoriaMetrics/skills).

# AI observability

VictoriaMetrics Observability Stack is optimal for monitoring AI agents using auto-instrumentation libraries
like [OpenLLMetry](https://github.com/traceloop/openllmetry), [OpenInference](https://github.com/Arize-ai/openinference),
[OpenLIT](https://victoriametrics.com/blog/ai-agents-observability/#using-openlit).
Please see more details in [AI Agents Observability with OpenTelemetry and the VictoriaMetrics Stack](https://victoriametrics.com/blog/ai-agents-observability).

AI code assistants like Claude Code, OpenAI Codex, Gemini CLI, Qwen Code, and OpenCode expose internal telemetry that
helps to monitor cost usage, analytics, performance, compliance and improves troubleshooting experience. All major
AI coding tools support OpenTelemetry and can be easily integrated into VictoriaMetrics Observability Stack.
Please see more details in [Vibe coding tools observability with VictoriaMetrics Stack and OpenTelemetry
](https://victoriametrics.com/blog/vibe-coding-observability/).
