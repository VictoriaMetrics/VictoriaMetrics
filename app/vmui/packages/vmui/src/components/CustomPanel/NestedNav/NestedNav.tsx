import React, {FC} from "preact/compat";
import Box from "@mui/material/Box";
import ListItem from "@mui/material/ListItem";
import ListItemText from "@mui/material/ListItemText";
import ListItemButton from "@mui/material/ListItemButton";
import ListItemIcon from "@mui/material/ListItemIcon";
import ExpandLess from "@mui/icons-material/ExpandLess";
import AddCircleRoundedIcon from "@mui/icons-material/AddCircleRounded";
import Collapse from "@mui/material/Collapse";
import List from "@mui/material/List";
import {BorderLinearProgressWithLabel} from "../../BorderLineProgress/BorderLinearProgress";
import Trace from "../Trace/Trace";

interface RecursiveProps {
  trace: Trace;
  totalMsec: number;
  openLevels: Record<number, boolean>;
  onChange: (level: number) => void;
}

const NestedNav: FC<RecursiveProps> = ({ trace, openLevels, totalMsec, onChange})  => {
  const handleListClick = (traceID: number) => () => onChange(traceID);
  const hasChildren = trace.children && trace.children.length;
  const progress = trace.duration / totalMsec * 100;
  return (
    <Box sx={{ bgcolor: "rgba(201, 227, 246, 0.4)" }}>
      <ListItem onClick={handleListClick(trace.duration)} sx={!hasChildren ? {p:0, pl: 7} : {p:0}}>
        <ListItemButton alignItems={"flex-start"} sx={{ pt: 0, pb: 0}}>
          {hasChildren ? <ListItemIcon>
            {openLevels[trace.duration] ?
              <ExpandLess fontSize={"large"} color={"info"} /> :
              <AddCircleRoundedIcon fontSize={"large"} color={"info"} />}
          </ListItemIcon>: null}
          <Box display="flex" flexDirection="column" flexGrow={0.5} sx={{ ml: 4, mr: 4, width: "100%" }}>
            <ListItemText
              primary={`duration: ${trace.duration} ms`}
              secondary={trace.message}
            />
            <ListItemText>
              <BorderLinearProgressWithLabel variant="determinate" value={progress} />
            </ListItemText>
          </Box>
        </ListItemButton>
      </ListItem>
      <>
        <Collapse in={openLevels[trace.duration]} timeout="auto" unmountOnExit>
          <List component="div" disablePadding sx={{ pl: 4 }}>
            {hasChildren ?
              trace.children.map((trace) => <NestedNav
                key={trace.duration}
                trace={trace}
                openLevels={openLevels}
                totalMsec={totalMsec}
                onChange={onChange}
              />) : null}
          </List>
        </Collapse>
      </>
    </Box>
  );
};

export default NestedNav;
