import React from "preact/compat";
import { styled } from "@mui/material/styles";
import LinearProgressWithLabel, {linearProgressClasses, LinearProgressProps} from "@mui/material/LinearProgress";
import {Box, Typography} from "@mui/material";

export const BorderLinearProgress = styled(LinearProgressWithLabel)(({ theme }) => ({
  height: 20,
  borderRadius: 5,
  [`&.${linearProgressClasses.colorPrimary}`]: {
    backgroundColor: theme.palette.grey[theme.palette.mode === "light" ? 200 : 800],
  },
  [`& .${linearProgressClasses.bar}`]: {
    borderRadius: 5,
    backgroundColor: theme.palette.mode === "light" ? "#1a90ff" : "#308fe8",
  },
}));

export const BorderLinearProgressWithLabel = (props: LinearProgressProps & { value: number }) => (
  <Box sx={{ display: "flex", alignItems: "center" }}>
    <Box sx={{ width: "100%", mr: 1 }}>
      <BorderLinearProgress variant="determinate" {...props} />
    </Box>
    <Box sx={{ minWidth: 35 }}>
      <Typography variant="body2" color="text.secondary">{`${props.value.toFixed(2)}%`}</Typography>
    </Box>
  </Box>
);
