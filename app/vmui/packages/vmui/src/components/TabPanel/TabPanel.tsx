import {ReactNode} from "react";
import {Box} from "@mui/material";
import React from "preact/compat";

interface TabPanelProps {
  children?: ReactNode;
  index: number;
  value: number;
}

const TabPanel = (props: TabPanelProps) => {
  const { children, value, index, ...other } = props;
  return (
    <div
      role="tabpanel"
      hidden={value !== index}
      id={`simple-tabpanel-${index}`}
      aria-labelledby={`simple-tab-${index}`}
      {...other}
    >
      {value === index && (<Box sx={{ p: 3 }}>{children}</Box>)}
    </div>
  );
};

export default TabPanel;
