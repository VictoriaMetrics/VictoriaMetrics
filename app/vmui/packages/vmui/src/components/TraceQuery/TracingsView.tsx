import React, { FC } from "preact/compat";
import TraceView from "./TraceView";
import Trace from "./Trace";
import Button from "../Main/Button/Button";
import { RemoveCircleIcon } from "../Main/Icons";

interface TraceViewProps {
  traces: Trace[];
  onDeleteClick: (trace: Trace) => void;
}

const TracingsView: FC<TraceViewProps> = ({ traces, onDeleteClick }) => {
  if (!traces.length) {
    return (
      // TODO add alert
      <div>
        Please re-run the query to see results of the tracing
      </div>
    );
  }

  const handleDeleteClick = (tracingData: Trace) => () => {
    onDeleteClick(tracingData);
  };

  return <>{traces.map((trace: Trace) => <>
    <div>
      Trace for <b>{trace.queryValue}</b>
      <Button onClick={handleDeleteClick(trace)}>
        <RemoveCircleIcon />
      </Button>
    </div>
    <TraceView trace={trace} />
  </>)}</>;
};

export default TracingsView;
