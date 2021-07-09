import React from "react";
import {Box, makeStyles, Typography} from "@material-ui/core";

export interface ChartTooltipData {
  value: number;
  metrics: {
    key: string;
    value: string;
  }[];
  color?: string;
}

export interface ChartTooltipProps {
  data: ChartTooltipData;
  time?: Date;
}

const useStyle = makeStyles(() => ({
  wrapper: {
    maxWidth: "40vw"
  }
}));

export const ChartTooltip: React.FC<ChartTooltipProps> = ({data, time}) => {
  const classes = useStyle();

  return (
    <Box px={1} className={classes.wrapper}>
      <Box fontStyle="italic" mb={.5}>
        <Typography variant="subtitle1">{`${time?.toLocaleDateString()} ${time?.toLocaleTimeString()}`}</Typography>
      </Box>
      <Box mb={.5} my={1}>
        <Typography variant="subtitle2">{`Value: ${new Intl.NumberFormat(undefined, {
          maximumFractionDigits: 10
        }).format(data.value)}`}</Typography>
      </Box>
      <Box>
        <Typography variant="body2">
          {data.metrics.map(({key, value}) =>
            <Box mb={.25} key={key} display="flex" flexDirection="row" alignItems="center">
              <span>{key}:&nbsp;</span>
              <span style={{fontWeight: "bold"}}>{value}</span>
            </Box>)}
        </Typography>
      </Box>
    </Box>
  );
};