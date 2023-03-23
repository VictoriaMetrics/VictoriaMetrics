import React, { FC } from "preact/compat";
import Hyperlink from "../../../components/Main/Hyperlink/Hyperlink";

const last_over_time = <Hyperlink
  text="last_over_time"
  href="https://docs.victoriametrics.com/MetricsQL.html#last_over_time"
  underlined
/>;

const instant_query = <Hyperlink
  text="instant query"
  href="https://docs.victoriametrics.com/MetricsQL.html#last_over_time"
  underlined
/>;

const InstantQueryTip: FC = () => (
  <div>
    <p>
      This tab use {instant_query} that tries to locate a data sample is equal to <b>5m</b>.
      We have intentionally done this to ensure compatibility with Prometheus.
    </p>
    <p>
      You can change the <b>time range</b> or use {last_over_time} .
    </p>
  </div>
);

export default InstantQueryTip;
