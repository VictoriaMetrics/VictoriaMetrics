import React, { FC } from "preact/compat";
import NestedNav from "./NestedNav/NestedNav";
import Trace from "./Trace";

interface TraceViewProps {
  trace: Trace;
}

const TraceView: FC<TraceViewProps> = ({ trace }) => {

  return (<nav>
    <NestedNav
      trace={trace}
      totalMsec={trace.duration}
    />
  </nav>);
};

export default TraceView;
