import React, {FC, useEffect, useState} from "preact/compat";
import {ChangeEvent} from "react";
import Box from "@mui/material/Box";
import FormControlLabel from "@mui/material/FormControlLabel";
import TextField from "@mui/material/TextField";
import BasicSwitch from "../../../../theme/switch";

interface StepConfiguratorProps {
  defaultStep?: number,
  customStepEnable: boolean,
  setStep: (step: number) => void,
  toggleEnableStep: () => void
}

const StepConfigurator: FC<StepConfiguratorProps> = ({
  defaultStep, customStepEnable, setStep, toggleEnableStep
}) => {

  const [customStep, setCustomStep] = useState(defaultStep);
  const [error, setError] = useState(false);

  useEffect(() => {
    setStep(customStep || 1);
  }, [customStep]);

  const onChangeStep = (e: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    if (!customStepEnable) return;
    const value = +e.target.value;
    if (value > 0) {
      setCustomStep(value);
      setError(false);
    } else {
      setError(true);
    }
  };

  const onChangeEnableStep = () => {
    setError(false);
    toggleEnableStep();
  };

  return <Box display="grid" gridTemplateColumns="auto 120px" alignItems="center">
    <FormControlLabel
      control={<BasicSwitch checked={customStepEnable} onChange={onChangeEnableStep}/>}
      label="Override step value" sx={{ml: 0}}
    />
    <TextField
      label="Step value"
      type="number"
      size="small"
      variant="outlined"
      value={customStep}
      disabled={!customStepEnable}
      error={error}
      helperText={error ? "step is out of allowed range" : " "}
      onChange={onChangeStep}/>
  </Box>;
};

export default StepConfigurator;
