import {TraceData} from "../../../api/types";
import Box from "@mui/material/Box";
import ListItem from "@mui/material/ListItem";
import ListItemText from "@mui/material/ListItemText";
import RemoveIcon from "@mui/icons-material/Remove";
import AddIcon from "@mui/icons-material/Add";
import Collapse from "@mui/material/Collapse";
import List from "@mui/material/List";
import React from "preact/compat";

interface RecursiveProps {
  traceData: TraceData;
  openLevels: Record<number, boolean>;
  onChange: (level: number) => void;
}

type RecursiveComponent = (props: RecursiveProps) => JSX.Element;

export const recursiveComponent: RecursiveComponent = ({ traceData, openLevels, onChange})  => {
  const {children} = traceData;
  const handleListClick = (level: number) => () => onChange(level);

  return (
    <Box sx={{ bgcolor: "rgba(227, 242, 253, 0.6)" }}>
      {children.map((child) => {
        const hasChildren = child.children && child.children.length;
        if (!hasChildren) {
          return (
            <>
              <ListItem>
                <ListItemText primary={child.duration_msec} secondaryTypographyProps={{align:"left"}}/>
                <ListItemText primary={child.message} secondaryTypographyProps={{align:"left"}}/>
              </ListItem>
            </>
          );
        }
        return (
          <>
            <ListItem onClick={handleListClick(child.duration_msec)}>
              {openLevels[child.duration_msec] ? <RemoveIcon /> : <AddIcon />}
              <ListItemText primary={child.duration_msec} secondaryTypographyProps={{align:"left"}}/>
              <ListItemText primary={child.message} secondaryTypographyProps={{align:"left"}}/>
            </ListItem>
            <Collapse in={openLevels[child.duration_msec]} timeout="auto" unmountOnExit>
              <List component="div" disablePadding sx={{ pl: 4 }}>
                {recursiveComponent({traceData: child, openLevels, onChange})}
              </List>
            </Collapse>
          </>
        );
      }).reverse()}
    </Box>
  );
};
