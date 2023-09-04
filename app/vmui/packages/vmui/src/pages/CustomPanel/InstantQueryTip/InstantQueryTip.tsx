import React, { FC } from "preact/compat";
import Hyperlink from "../../../components/Main/Hyperlink/Hyperlink";

const last_over_time = <Hyperlink
  text="last_over_time"
  href="https://docs.victoriametrics.com/MetricsQL.html#last_over_time"
  underlined
/>;

const instant_query = <Hyperlink
  text="instant query"
  href="https://docs.victoriametrics.com/keyConcepts.html#instant-query"
  underlined
/>;

const InstantQueryTip: FC = () => (
  <div>
    <p>
      This tab shows {instant_query} results for the last 5 minutes ending at the selected time range.
    </p>
    <p>
      Please wrap the query into {last_over_time} if you need results over arbitrary lookbehind interval.
    </p>
  </div>
);

export default InstantQueryTip;
