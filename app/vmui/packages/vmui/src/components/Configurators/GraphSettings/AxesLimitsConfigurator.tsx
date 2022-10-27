import React, { FC, useCallback, useMemo } from "preact/compat";
import { ChangeEvent } from "react";
import Box from "@mui/material/Box";
import FormControlLabel from "@mui/material/FormControlLabel";
import TextField from "@mui/material/TextField";
import debounce from "lodash.debounce";
import BasicSwitch from "../../../theme/switch";
import { AxisRange, YaxisState } from "../../../state/graph/reducer";

interface AxesLimitsConfiguratorProps {
  yaxis: YaxisState,
  setYaxisLimits: (limits: AxisRange) => void,
  toggleEnableLimits: () => void
}

const AxesLimitsConfigurator: FC<AxesLimitsConfiguratorProps> = ({ yaxis, setYaxisLimits, toggleEnableLimits }) => {

  const axes = useMemo(() => Object.keys(yaxis.limits.range), [yaxis.limits.range]);

  const onChangeLimit = (e: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>, axis: string, index: number) => {
    const newLimits = yaxis.limits.range;
    newLimits[axis][index] = +e.target.value;
    if (newLimits[axis][0] === newLimits[axis][1] || newLimits[axis][0] > newLimits[axis][1]) return;
    setYaxisLimits(newLimits);
  };
  const debouncedOnChangeLimit = useCallback(debounce(onChangeLimit, 500), [yaxis.limits.range]);

  return <Box
    display="grid"
    alignItems="center"
    gap={2}
  >
    <FormControlLabel
      control={<BasicSwitch
        checked={yaxis.limits.enable}
        onChange={toggleEnableLimits}
      />}
      label="Fix the limits for y-axis"
    />
    <Box
      display="grid"
      alignItems="center"
      gap={2}
    >
      {axes.map(axis => <Box
        display="grid"
        gridTemplateColumns="120px 120px"
        gap={1}
        key={axis}
      >
        <TextField
          label={`Min ${axis}`}
          type="number"
          size="small"
          variant="outlined"
          disabled={!yaxis.limits.enable}
          defaultValue={yaxis.limits.range[axis][0]}
          onChange={(e) => debouncedOnChangeLimit(e, axis, 0)}
        />
        <TextField
          label={`Max ${axis}`}
          type="number"
          size="small"
          variant="outlined"
          disabled={!yaxis.limits.enable}
          defaultValue={yaxis.limits.range[axis][1]}
          onChange={(e) => debouncedOnChangeLimit(e, axis, 1)}
        />
      </Box>)}
    </Box>
  </Box>;
};

export default AxesLimitsConfigurator;
