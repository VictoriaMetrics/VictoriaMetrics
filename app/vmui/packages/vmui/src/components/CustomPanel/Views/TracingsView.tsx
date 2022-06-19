import React, {FC} from "preact/compat";
import {TracingData} from "../../../api/types";
import Typography from "@mui/material/Typography";
import TraceView from "./TraceView";
import Alert from "@mui/material/Alert";

interface TraceViewProps {
  tracingsData: TracingData[];
  emptyMessage: string;
}

const TracingsView: FC<TraceViewProps> = ({tracingsData, emptyMessage}) => {
  if (!tracingsData.length) {
    return (
      <>
        <Alert color={"info"} severity="info" sx={{whiteSpace: "pre-wrap", mt: 2}}>
          {emptyMessage}
        </Alert>
      </>
    );
  }
  return <>{tracingsData.map((tracingData, idx) => <>
    <Typography variant="h4" gutterBottom component="div">{`Query ${idx+1} tracing`}</Typography>
    <TraceView tracingData={tracingData}/>
  </>)}</>;
};

export default TracingsView;
