import React, {FC, useState} from "preact/compat";
import {TraceData} from "../../../api/types";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemText from "@mui/material/ListItemText";
import Collapse from "@mui/material/Collapse";
import Typography from "@mui/material/Typography";
import Box from "@mui/material/Box";
import AddIcon from "@mui/icons-material/Add";
import RemoveIcon from "@mui/icons-material/Remove";
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
    <Box sx={{ bgcolor: "rgba(227, 242, 253, 0.6)" }}>
      <ListItem onClick={() => handleClick(traceData.duration_msec)}>
        {openLevels[traceData.duration_msec] ? <RemoveIcon /> : <AddIcon />}
        <ListItemText primary={traceData.duration_msec} />
        <ListItemText secondary={traceData.message} secondaryTypographyProps={{align: "left"}}/>
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
