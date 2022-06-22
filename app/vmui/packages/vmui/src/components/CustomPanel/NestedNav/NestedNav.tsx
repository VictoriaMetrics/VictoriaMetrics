import React from "preact/compat";
import {TracingData} from "../../../api/types";
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

interface RecursiveProps {
  tracingData: TracingData[];
  totalMicrosec: number;
  openLevels: Record<number, boolean>;
  onChange: (level: number) => void;
}

type RecursiveComponent = (props: RecursiveProps) => JSX.Element;

export const recursiveComponent: RecursiveComponent = ({ tracingData, openLevels, totalMicrosec, onChange})  => {
  const handleListClick = (level: number) => () => onChange(level);
  return (
    <Box sx={{ bgcolor: "rgba(201, 227, 246, 0.4)" }}>
      {tracingData.map((child) => {
        const hasChildren = child.children && child.children.length;
        const progress = child.duration_msec / totalMicrosec * 100;
        if (!hasChildren) {
          return (
            <ListItem sx={{p:0, pl:7}}>
              <ListItemButton sx={{ pt: 0, pb: 0}}>
                <Box display="flex" flexDirection="column" flexGrow={0.5} sx={{ ml: 4, mr: 4, width: "100%" }}>
                  <ListItemText
                    primary={`duration: ${child.duration_msec} ms`}
                    secondary={child.message} />
                  <ListItemText>
                    <BorderLinearProgressWithLabel variant="determinate" value={progress} />
                  </ListItemText>
                </Box>
              </ListItemButton>
            </ListItem>
          );
        }
        return (
          <>
            <ListItem onClick={handleListClick(child.duration_msec)} sx={{p:0}}>
              <ListItemButton alignItems={"flex-start"} sx={{ pt: 0, pb: 0}}>
                <ListItemIcon>
                  {openLevels[child.duration_msec] ?
                    <ExpandLess fontSize={"large"} color={"info"} /> :
                    <AddCircleRoundedIcon fontSize={"large"} color={"info"} />}
                </ListItemIcon>
                <Box display="flex" flexDirection="column" flexGrow={0.5} sx={{ ml: 4, mr: 4, width: "100%" }}>
                  <ListItemText
                    primary={`duration: ${child.duration_msec} ms`}
                    secondary={child.message}
                  />
                  <ListItemText>
                    <BorderLinearProgressWithLabel variant="determinate" value={progress} />
                  </ListItemText>
                </Box>
              </ListItemButton>
            </ListItem>
            <Collapse in={openLevels[child.duration_msec]} timeout="auto" unmountOnExit>
              <List component="div" disablePadding sx={{ pl: 4 }}>
                {recursiveComponent({tracingData: child.children, openLevels, totalMicrosec, onChange})}
              </List>
            </Collapse>
          </>
        );
      })}
    </Box>
  );
};
