import React, {FC} from "preact/compat";
import Typography from "@mui/material/Typography";
import TraceView from "./TraceView";
import Alert from "@mui/material/Alert";
import RemoveCircleIcon from "@mui/icons-material/RemoveCircle";
import Button from "@mui/material/Button";
import Trace from "../Trace/Trace";

interface TraceViewProps {
  traces: Trace[];
  onDeleteClick: (trace: Trace) => void;
}

const TracingsView: FC<TraceViewProps> = ({traces, onDeleteClick}) => {
  if (!traces.length) {
    return (
      <Alert color={"info"} severity="info" sx={{whiteSpace: "pre-wrap", mt: 2}}>
        Please re-run the query to see results of the tracing
      </Alert>
    );
  }

  const handleDeleteClick = (tracingData: Trace) => () => {
    onDeleteClick(tracingData);
  };

  return <>{traces.map((trace: Trace) => <>
    <Typography variant="h5" component="div">
      Trace for <b>{trace.queryValue}</b>
      <Button onClick={handleDeleteClick(trace)}>
        <RemoveCircleIcon fontSize={"medium"} color={"error"} />
      </Button>
    </Typography>
    <TraceView trace={trace} />
  </>)}</>;
};

export default TracingsView;
