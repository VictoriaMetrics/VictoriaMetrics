import React, { FC } from "preact/compat";
import "./style.scss";
import CodeExample from "../../../components/Main/CodeExample/CodeExample";

const MetricsQL = () => (
  <a
    className="vm-link vm-link_colored"
    href="https://docs.victoriametrics.com/MetricsQL.html"
    target="_blank"
    rel="help noreferrer"
  >
    MetricsQL
  </a>
);

const NodeExporterFull = () => (
  <a
    className="vm-link vm-link_colored"
    href="https://grafana.com/grafana/dashboards/1860-node-exporter-full/"
    target="_blank"
    rel="help noreferrer"
  >
    Node Exporter Full
  </a>
);

const WithTemplateTutorial: FC = () => (
  <section className="vm-with-template-tutorial">
    <h2 className="vm-with-template-tutorial__title">
      Tutorial for WITH expressions in <MetricsQL/>
    </h2>
    <div className="vm-with-template-tutorial-section">
      <p className="vm-with-template-tutorial-section__text">Let&apos;s look at the following real query from <NodeExporterFull/> dashboard:</p>
      <CodeExample
        code= {`(
  (
    node_memory_MemTotal_bytes{instance=~"$node:$port", job=~"$job"}
      -
    node_memory_MemFree_bytes{instance=~"$node:$port", job=~"$job"}
  )
    /
  node_memory_MemTotal_bytes{instance=~"$node:$port", job=~"$job"}
) * 100`}
      />
      <p className="vm-with-template-tutorial-section__text">
        It is clear the query calculates the percentage of used memory for the given $node, $port and $job.
        Isn&apos;t it? :)
      </p>
    </div>
    <div className="vm-with-template-tutorial-section">
      <p className="vm-with-template-tutorial-section__text">
        What&apos;s wrong with this query?
        Copy-pasted label filters for distinct timeseries which makes it easy
        to mistype these filters during modification. Let&apos;s simplify the query with WITH expressions:
      </p>
      <CodeExample
        code={`WITH (
    commonFilters = {instance=~"$node:$port",job=~"$job"}
)
(
  node_memory_MemTotal_bytes{commonFilters}
    -
  node_memory_MemFree_bytes{commonFilters}
)
  /
node_memory_MemTotal_bytes{commonFilters} * 100`}
      />
    </div>
    <div className="vm-with-template-tutorial-section">
      <p className="vm-with-template-tutorial-section__text">
        Now label filters are located in a single place instead of three distinct places.
        The query mentions node_memory_MemTotal_bytes metric twice and {"{commonFilters}"} three times.
        WITH expressions may improve this:
      </p>
      <CodeExample
        code={`WITH (
    my_resource_utilization(free, limit, filters) = (limit{filters} - free{filters}) / limit{filters} * 100
)
my_resource_utilization(
  node_memory_MemFree_bytes,
  node_memory_MemTotal_bytes,
  {instance=~"$node:$port",job=~"$job"},
)`}
      />
      <p className="vm-with-template-tutorial-section__text">
        Now the template function my_resource_utilization() may be used
        for monitoring arbitrary resources - memory, CPU, network, storage, you name it.
      </p>
    </div>
    <div className="vm-with-template-tutorial-section">
      <p className="vm-with-template-tutorial-section__text">
        Let&apos;s take another nice query from <NodeExporterFull/> dashboard:
      </p>
      <CodeExample
        code={`(
  (
    (
      count(
        count(node_cpu_seconds_total{instance=~"$node:$port",job=~"$job"}) by (cpu)
      )
    )
      -
    avg(
      sum by (mode) (rate(node_cpu_seconds_total{mode='idle',instance=~"$node:$port",job=~"$job"}[5m]))
    )
  )
    *
  100
)
  /
count(
  count(node_cpu_seconds_total{instance=~"$node:$port",job=~"$job"}) by (cpu)
)`}
      />
      <p className="vm-with-template-tutorial-section__text">
        Do you understand what does this mess do? Is it manageable? :)
        WITH expressions are happy to help in a few iterations.
      </p>
    </div>

    <div className="vm-with-template-tutorial-section">
      <p className="vm-with-template-tutorial-section__text">
        1. Extract common filters used in multiple places into a commonFilters variable:
      </p>
      <CodeExample
        code={`WITH (
    commonFilters = {instance=~"$node:$port",job=~"$job"}
)
(
  (
    (
      count(
        count(node_cpu_seconds_total{commonFilters}) by (cpu)
      )
    )
      -
    avg(
      sum by (mode) (rate(node_cpu_seconds_total{mode='idle',commonFilters}[5m]))
    )
  )
    *
  100
)
  /
count(
  count(node_cpu_seconds_total{commonFilters}) by (cpu)
)`}
      />
    </div>
    <div className="vm-with-template-tutorial-section">
      <p className="vm-with-template-tutorial-section__text">
        2. Extract &#34;count(count(...) by (cpu))&#34; into cpuCount variable:
      </p>
      <CodeExample
        code={`WITH (
    commonFilters = {instance=~"$node:$port",job=~"$job"},
    cpuCount = count(count(node_cpu_seconds_total{commonFilters}) by (cpu))
)
(
  (
    cpuCount
      -
    avg(
      sum by (mode) (rate(node_cpu_seconds_total{mode='idle',commonFilters}[5m]))
    )
  )
    *
  100
) / cpuCount`}
      />
    </div>
    <div className="vm-with-template-tutorial-section">
      <p className="vm-with-template-tutorial-section__text">
        3. Extract rate(...) part into cpuIdle variable,
        since it is clear now that this part calculates the number of idle CPUs:
      </p>
      <CodeExample
        code={`WITH (
    commonFilters = {instance=~"$node:$port",job=~"$job"},
    cpuCount = count(count(node_cpu_seconds_total{commonFilters}) by (cpu)),
    cpuIdle = sum(rate(node_cpu_seconds_total{mode='idle',commonFilters}[5m]))
)
((cpuCount - cpuIdle) * 100) / cpuCount`}
      />
    </div>
    <div className="vm-with-template-tutorial-section">
      <p className="vm-with-template-tutorial-section__text">
        4. Put node_cpu_seconds_total{"{commonFilters}"} into its own varialbe with the name cpuSeconds:
      </p>
      <CodeExample
        code={`WITH (
    cpuSeconds = node_cpu_seconds_total{instance=~"$node:$port",job=~"$job"},
    cpuCount = count(count(cpuSeconds) by (cpu)),
    cpuIdle = sum(rate(cpuSeconds{mode='idle'}[5m]))
)
((cpuCount - cpuIdle) * 100) / cpuCount`}
      />
      <p className="vm-with-template-tutorial-section__text">
        Now the query became more clear comparing to the initial query.
      </p>
    </div>
    <div className="vm-with-template-tutorial-section">
      <p className="vm-with-template-tutorial-section__text">
        WITH expressions may be nested and may be put anywhere. Try expanding the following query:
      </p>
      <CodeExample
        code= {`WITH (
    f(a, b) = WITH (
        f1(x) = b-x,
        f2(x) = x+x
    ) f1(a)*f2(b)
) f(foo, with(x=bar) x)`}
      />
    </div>
  </section>
);

export default WithTemplateTutorial;
