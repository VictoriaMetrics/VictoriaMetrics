import React, {FC, useState} from "preact/compat";
import List from "@mui/material/List";
import RecursiveComponent from "../NestedNav/NestedNav";
import Trace from "../Trace/Trace";

interface TraceViewProps {
  trace: Trace;
}

interface OpenLevels {
  [x: number]: boolean
}

const TraceView: FC<TraceViewProps> = ({trace}) => {

  const [openLevels, setOpenLevels] = useState({} as OpenLevels);
  const handleClick = (level: number) => {
    setOpenLevels((prevState:OpenLevels) => ({...prevState, [level]: !prevState[level]}));
  };
  return (<List sx={{ width: "100%" }} component="nav">
    <RecursiveComponent
      trace={trace}
      openLevels={openLevels}
      totalMicrosec={trace.duration}
      onChange={handleClick}
    />
  </List>);
};

export default TraceView;
