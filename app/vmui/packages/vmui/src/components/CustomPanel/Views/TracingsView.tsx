import React, {FC} from "preact/compat";
import Typography from "@mui/material/Typography";
import TraceView from "./TraceView";
import Alert from "@mui/material/Alert";
import RemoveCircleIcon from "@mui/icons-material/RemoveCircle";
import Button from "@mui/material/Button";
import Trace from "../Trace/Trace";

interface TraceViewProps {
  tracingsData: Trace[];
  onDeleteClick: (tracingData: Trace) => void;
}

const EMPTY_MESSAGE = "Please re-run the query to see results of the tracing";

const TracingsView: FC<TraceViewProps> = ({tracingsData, onDeleteClick}) => {
  if (!tracingsData.length) {
    return (
      <Alert color={"info"} severity="info" sx={{whiteSpace: "pre-wrap", mt: 2}}>
        {EMPTY_MESSAGE}
      </Alert>
    );
  }

  const handleDeleteClick = (tracingData: Trace) => () => {
    onDeleteClick(tracingData);
  };

  return <>{tracingsData.map((tracingData) => <>
    <Typography variant="h4" gutterBottom component="div">
      {"Tracing for"} {tracingData.queryValue}
      <Button onClick={handleDeleteClick(tracingData)}>
        <RemoveCircleIcon fontSize={"large"} color={"error"} />
      </Button>
    </Typography>
    <TraceView tracingData={tracingData} />
  </>)}</>;
};

export default TracingsView;
