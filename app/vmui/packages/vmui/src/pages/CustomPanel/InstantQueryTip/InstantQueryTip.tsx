import React, { FC } from "preact/compat";
import Hyperlink from "../../../components/Main/Hyperlink/Hyperlink";
import { useGraphState } from "../../../state/graph/GraphStateContext";

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

const InstantQueryTip: FC = () => {
  const { customStep } = useGraphState();

  return (
    <div>
      <p>
        This tab shows {instant_query} results for the last {customStep || "5m"} (defined by the <code>step</code>) ending at the selected time range.
      </p>
      <p>
        Please wrap the query into {last_over_time} if you need results over arbitrary lookbehind interval.
      </p>
    </div>
  );
};

export default InstantQueryTip;
