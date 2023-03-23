import React, { FC } from "preact/compat";

const InstantQueryTip: FC = () => {

  return (
    <div>
      <p>
        This tab use <a
          className="vm-link vm-link_colored vm-link_underlined"
          href="https://docs.victoriametrics.com/keyConcepts.html#instant-query"
          target="_blank"
          rel="help noreferrer"
        >instant query</a> that tries to locate a data sample is equal to <b>5m</b>.
        We have intentionally done this to ensure compatibility with Prometheus.
      </p>
      <p>
        You can change the <b>time range</b> or use <a
          className="vm-link vm-link_colored vm-link_underlined"
          href="https://docs.victoriametrics.com/MetricsQL.html#last_over_time"
          target="_blank"
          rel="help noreferrer"
        >last_over_time</a>.
      </p>
    </div>
  );
};

export default InstantQueryTip;
