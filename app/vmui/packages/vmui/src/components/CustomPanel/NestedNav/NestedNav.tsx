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
import React from "preact/compat";

interface RecursiveProps {
  tracingData: TracingData;
  openLevels: Record<number, boolean>;
  onChange: (level: number) => void;
}

type RecursiveComponent = (props: RecursiveProps) => JSX.Element;

export const recursiveComponent: RecursiveComponent = ({ tracingData, openLevels, onChange})  => {
  const {children} = tracingData;
  const handleListClick = (level: number) => () => onChange(level);
  const renderData = children.sort((a, b) => {
    if (a.duration_msec > b.duration_msec) return 1;
    if (a.duration_msec < b.duration_msec) return -1;
    return 0;
  });
  return (
    <Box sx={{ bgcolor: "rgba(201, 227, 246, 0.4)" }}>
      {renderData.map((child) => {
        const hasChildren = child.children && child.children.length;
        if (!hasChildren) {
          return (
            <>
              <ListItem sx={{p:0, pl:7}}>
                <ListItemButton sx={{ pt: 0, pb: 0}}>
                  <ListItemText
                    primary={`duration: ${child.duration_msec} ms`}
                    secondary={child.message} />
                </ListItemButton>
              </ListItem>
            </>
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
                <ListItemText
                  primary={`duration: ${child.duration_msec} ms`}
                  secondary={child.message} />
              </ListItemButton>
            </ListItem>
            <Collapse in={openLevels[child.duration_msec]} timeout="auto" unmountOnExit>
              <List component="div" disablePadding sx={{ pl: 4 }}>
                {recursiveComponent({tracingData: child, openLevels, onChange})}
              </List>
            </Collapse>
          </>
        );
      }).reverse()}
    </Box>
  );
};
