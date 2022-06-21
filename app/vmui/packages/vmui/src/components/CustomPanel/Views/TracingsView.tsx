import React, {FC} from "preact/compat";
import {TracingData} from "../../../api/types";
import Typography from "@mui/material/Typography";
import TraceView from "./TraceView";
import Alert from "@mui/material/Alert";
import RemoveCircleIcon from "@mui/icons-material/RemoveCircle";
import Button from "@mui/material/Button";

interface TraceViewProps {
  tracingsData: TracingData[];
  emptyMessage: string;
  onDeleteClick: (tracingData: TracingData) => void;
}

const TracingsView: FC<TraceViewProps> = ({tracingsData, emptyMessage, onDeleteClick}) => {
  if (!tracingsData.length) {
    return (
      <>
        <Alert color={"info"} severity="info" sx={{whiteSpace: "pre-wrap", mt: 2}}>
          {emptyMessage}
        </Alert>
      </>
    );
  }

  const handleDeleteClick = (tracingData: TracingData) => () => {
    onDeleteClick(tracingData);
  };

  const getQuery = (message: string): string => {
    const query = message.match(/query=(.*):/);
    if (query) return query[1];
    return "";
  };

  return <>{tracingsData.map((tracingData) => <>
    <Typography variant="h4" gutterBottom component="div">
      {`Query ${getQuery(tracingData.message)} tracing`}
      <Button onClick={handleDeleteClick(tracingData)}>
        <RemoveCircleIcon fontSize={"large"} color={"error"} />
      </Button>
    </Typography>
    <TraceView tracingData={tracingData} />
  </>)}</>;
};

export default TracingsView;
