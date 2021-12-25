import React from "preact/compat";
import Box from "@mui/material/Box";

interface TabPanelProps {
  index: number;
  value: number;
}

const AuthTabPanel: React.FC<TabPanelProps> = (props) => {
  const { children, value, index, ...other } = props;

  return (
    <div
      role="tabpanel"
      hidden={value !== index}
      id={`auth-config-tabpanel-${index}`}
      aria-labelledby={`auth-config-tab-${index}`}
      {...other}
    >
      {value === index && (
        <Box py={2}>
          {children}
        </Box>
      )}
    </div>
  );
};

export default AuthTabPanel;
