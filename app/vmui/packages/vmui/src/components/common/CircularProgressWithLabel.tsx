import CircularProgress, {CircularProgressProps} from "@material-ui/core/CircularProgress";
import {Box} from "@material-ui/core";
import Typography from "@material-ui/core/Typography";
import React, {FC} from "react";

const CircularProgressWithLabel: FC<CircularProgressProps & { label: number }> = (props) => {
  return (
    <Box position="relative" display="inline-flex">
      <CircularProgress variant="determinate" {...props} />
      <Box
        top={0}
        left={0}
        bottom={0}
        right={0}
        position="absolute"
        display="flex"
        alignItems="center"
        justifyContent="center"
      >
        <Typography variant="caption" component="div">{`${props.label}s`}</Typography>
      </Box>
    </Box>
  );
};

export default CircularProgressWithLabel;