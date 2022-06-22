import React, {FC, useState} from "preact/compat";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemText from "@mui/material/ListItemText";
import ListItemIcon from "@mui/material/ListItemIcon";
import ListItemButton from "@mui/material/ListItemButton";
import Collapse from "@mui/material/Collapse";
import Box from "@mui/material/Box";
import ExpandLess from "@mui/icons-material/ExpandLess";
import AddCircleRoundedIcon from "@mui/icons-material/AddCircleRounded";
import {recursiveComponent} from "../NestedNav/NestedNav";
import Trace from "../Trace/Trace";


interface TraceViewProps {
  tracingData: Trace;
}

interface OpenLevels {
  [x: number]: boolean
}

const TraceView: FC<TraceViewProps> = ({tracingData}) => {

  const [openLevels, setOpenLevels] = useState({} as OpenLevels);
  const handleClick = (param: number) => {
    setOpenLevels((prevState:OpenLevels) => ({...prevState, [param]: !prevState[param]}));
  };

  return (<List sx={{ width: "100%" }} component="nav">
    <Box sx={{ bgcolor: "rgba(201, 227, 246, 0.4)" }}>
      <ListItem onClick={() => handleClick(tracingData.tracingValue.duration_msec)} sx={{p: 0}}>
        <ListItemButton sx={{ pt: 0, pb: 0}}>
          <ListItemIcon>
            {openLevels[tracingData.tracingValue.duration_msec] ?
              <ExpandLess fontSize={"large"} color={"info"} /> :
              <AddCircleRoundedIcon fontSize={"large"} color={"info"} />}
          </ListItemIcon>
          <ListItemText
            primary={`duration: ${tracingData.tracingValue.duration_msec} ms`}
            secondary={tracingData.tracingValue.message}
          />
        </ListItemButton>
      </ListItem>
      <Collapse in={openLevels[tracingData.tracingValue.duration_msec]} timeout="auto" unmountOnExit>
        <List component="div" disablePadding sx={{ pl: 4 }}>
          {recursiveComponent({
            tracingData: tracingData.tracingValue.children,
            openLevels,
            totalMicrosec: tracingData.tracingValue.duration_msec,
            onChange: handleClick
          })}
        </List>
      </Collapse>
    </Box>
  </List>);
};

export default TraceView;
