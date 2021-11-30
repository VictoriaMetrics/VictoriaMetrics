import React, {FC, useCallback, useMemo} from "react";
import {Box, FormControlLabel, Switch, TextField} from "@mui/material";
import {useGraphDispatch, useGraphState} from "../../../../state/graph/GraphStateContext";
import debounce from "lodash.debounce";

const AxesLimitsConfigurator: FC = () => {

  const { yaxis } = useGraphState();
  const graphDispatch = useGraphDispatch();
  const axes = useMemo(() => Object.keys(yaxis.limits.range), [yaxis.limits.range]);

  const onChangeYaxisLimits = () => { graphDispatch({type: "TOGGLE_ENABLE_YAXIS_LIMITS"}); };

  const onChangeLimit = (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>, axis: string, index: number) => {
    const newLimits = yaxis.limits.range;
    newLimits[axis][index] = +e.target.value;
    graphDispatch({type: "SET_YAXIS_LIMITS", payload: newLimits});
  };
  const debouncedOnChangeLimit = useCallback(debounce(onChangeLimit, 500), [yaxis.limits.range]);



  return <Box display="flex" alignItems="center" minHeight="50px" px={3}>
    <FormControlLabel
      control={<Switch size="small" checked={yaxis.limits.enable} onChange={onChangeYaxisLimits}/>}
      label="Fix the limits for y-axis"
    />
    <Box display="flex" alignItems="center" flexGrow={12}>
      {yaxis.limits.enable && axes.map(axis =>
        <Box display="grid" gridTemplateColumns="120px 120px" gap={1} mr={4} key={axis}>
          <TextField label={`Min ${axis}`} type="number" size="small" variant="outlined"
            defaultValue={yaxis.limits.range[axis][0]}
            onChange={(e) => debouncedOnChangeLimit(e, axis, 0)} />
          <TextField label={`Max ${axis}`} type="number" size="small" variant="outlined"
            defaultValue={yaxis.limits.range[axis][1]}
            onChange={(e) => debouncedOnChangeLimit(e, axis, 1)} />
        </Box>
      )}
    </Box>
  </Box>;
};

export default AxesLimitsConfigurator;