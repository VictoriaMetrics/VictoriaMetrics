import React, {FC} from "preact/compat";
import List from "@mui/material/List";
import NestedNav from "../NestedNav/NestedNav";
import Trace from "../Trace/Trace";

interface TraceViewProps {
  trace: Trace;
}

const TraceView: FC<TraceViewProps> = ({trace}) => {

  return (<List sx={{ width: "100%" }} component="nav">
    <NestedNav trace={trace} totalMsec={trace.duration} />
  </List>);
};

export default TraceView;
