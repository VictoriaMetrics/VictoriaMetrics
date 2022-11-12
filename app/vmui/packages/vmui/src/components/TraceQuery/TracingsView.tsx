import React, { FC } from "preact/compat";
import Trace from "./Trace";
import Button from "../Main/Button/Button";
import { DeleteIcon, RemoveCircleIcon } from "../Main/Icons";
import "./style.scss";
import NestedNav from "./NestedNav/NestedNav";
import Alert from "../Main/Alert/Alert";

interface TraceViewProps {
  traces: Trace[];
  onDeleteClick: (trace: Trace) => void;
}

const TracingsView: FC<TraceViewProps> = ({ traces, onDeleteClick }) => {
  if (!traces.length) {
    return (
      <Alert variant="info">
        Please re-run the query to see results of the tracing
      </Alert>
    );
  }

  const handleDeleteClick = (tracingData: Trace) => () => {
    onDeleteClick(tracingData);
  };

  return <div className="vm-tracings-view">
    {traces.map((trace: Trace) => (
      <div
        className="vm-tracings-view-trace vm-block vm-block_empty-padding"
        key={trace.idValue}
      >
        <div className="vm-tracings-view-trace-header">
          <h3 className="vm-tracings-view-trace-header-title">
            Trace for <b className="vm-tracings-view-trace-header-title__query">{trace.queryValue}</b>
          </h3>
          <Button
            variant="text"
            color="error"
            startIcon={<DeleteIcon/>}
            onClick={handleDeleteClick(trace)}
          />
        </div>
        <nav className="vm-tracings-view-trace__nav">
          <NestedNav
            trace={trace}
            totalMsec={trace.duration}
          />
        </nav>
      </div>
    ))}
  </div>;
};

export default TracingsView;
