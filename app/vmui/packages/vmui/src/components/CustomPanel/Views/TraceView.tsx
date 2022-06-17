import React, {FC, useState} from "preact/compat";
import {TraceData} from "../../../api/types";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemText from "@mui/material/ListItemText";
import ListItemIcon from "@mui/material/ListItemIcon";
import ListItemButton from "@mui/material/ListItemButton";
import Collapse from "@mui/material/Collapse";
import Typography from "@mui/material/Typography";
import Box from "@mui/material/Box";
import ExpandLess from "@mui/icons-material/ExpandLess";
import AddCircleRoundedIcon from "@mui/icons-material/AddCircleRounded";
import {recursiveComponent} from "../NestedNav/NestedNav";


interface TraceViewProps {
  traceData: TraceData;
}

interface OpenLevels {
  [x: number]: boolean
}

const TraceView: FC<TraceViewProps> = ({traceData}) => {
  const [openLevels, setOpenLevels] = useState({} as OpenLevels);
  const handleClick = (param: number) => {
    setOpenLevels((prevState:OpenLevels) => ({...prevState, [param]: !prevState[param]}));
  };

  return (<List sx={{ width: "100%" }} component="nav">
    <Typography variant="h4" gutterBottom component="div">Query tracing</Typography>
    <Box sx={{ bgcolor: "rgba(201, 227, 246, 0.4)" }}>
      <ListItem onClick={() => handleClick(traceData.duration_msec)}>
        <ListItemButton>
          <ListItemIcon>
            {openLevels[traceData.duration_msec] ?
              <ExpandLess fontSize={"large"} color={"info"} /> :
              <AddCircleRoundedIcon fontSize={"large"} color={"info"} />}
          </ListItemIcon>
          <ListItemText
            primary={`duration: ${traceData.duration_msec} ms`}
            secondary={traceData.message} />
        </ListItemButton>
      </ListItem>
      <Collapse in={openLevels[traceData.duration_msec]} timeout="auto" unmountOnExit>
        <List component="div" disablePadding sx={{ pl: 4 }}>
          {recursiveComponent({ traceData, openLevels, onChange: handleClick })}
        </List>
      </Collapse>
    </Box>
  </List>);
};

export default TraceView;
