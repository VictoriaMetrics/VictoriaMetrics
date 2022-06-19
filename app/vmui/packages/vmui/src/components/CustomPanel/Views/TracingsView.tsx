import React, {FC} from "preact/compat";
import {TracingData} from "../../../api/types";
import Typography from "@mui/material/Typography";
import TraceView from "./TraceView";
import Alert from "@mui/material/Alert";

interface TraceViewProps {
  tracingsData: TracingData[];
}

const TracingsView: FC<TraceViewProps> = ({tracingsData}) => {
  if (!tracingsData.length) {
    return (
      <>
        <Alert color={"info"} severity="info" sx={{whiteSpace: "pre-wrap", mt: 2}}>
          Please re-run the query to see results of the tracing
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
