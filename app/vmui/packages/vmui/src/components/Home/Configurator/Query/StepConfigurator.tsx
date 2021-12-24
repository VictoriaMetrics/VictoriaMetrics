import React, {FC, useCallback, useEffect, useState} from "preact/compat";
import {ChangeEvent} from "react";
import Box from "@mui/material/Box";
import FormControlLabel from "@mui/material/FormControlLabel";
import TextField from "@mui/material/TextField";
import BasicSwitch from "../../../../theme/switch";
import {useGraphDispatch, useGraphState} from "../../../../state/graph/GraphStateContext";
import {useAppState} from "../../../../state/common/StateContext";
import debounce from "lodash.debounce";

const StepConfigurator: FC = () => {
  const {customStep} = useGraphState();
  const graphDispatch = useGraphDispatch();
  const [error, setError] = useState(false);
  const {time: {period: {step}}} = useAppState();

  const onChangeStep = (e: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    const value = +e.target.value;
    if (value > 0) {
      graphDispatch({type: "SET_CUSTOM_STEP", payload: value});
      setError(false);
    } else {
      setError(true);
    }
  };

  const debouncedOnChangeStep = useCallback(debounce(onChangeStep, 500), [customStep.value]);

  const onChangeEnableStep = () => {
    setError(false);
    graphDispatch({type: "TOGGLE_CUSTOM_STEP"});
  };

  useEffect(() => {
    if (!customStep.enable) graphDispatch({type: "SET_CUSTOM_STEP", payload: step || 1});
  }, [step]);

  return <Box display="grid" gridTemplateColumns="auto 120px" alignItems="center">
    <FormControlLabel
      control={<BasicSwitch checked={customStep.enable} onChange={onChangeEnableStep}/>}
      label="Override step value"
    />
    {customStep.enable &&
      <TextField label="Step value" type="number" size="small" variant="outlined"
        defaultValue={customStep.value}
        error={error}
        helperText={error ? "step is out of allowed range" : " "}
        onChange={debouncedOnChangeStep}/>
    }
  </Box>;
};

export default StepConfigurator;