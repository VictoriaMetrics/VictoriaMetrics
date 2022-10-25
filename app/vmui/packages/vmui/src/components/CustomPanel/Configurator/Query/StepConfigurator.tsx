import React, {FC, useCallback, useState} from "preact/compat";
import {ChangeEvent} from "react";
import TextField from "@mui/material/TextField";
import debounce from "lodash.debounce";

interface StepConfiguratorProps {
  defaultStep?: number,
  setStep: (step: number) => void,
}

const StepConfigurator: FC<StepConfiguratorProps> = ({defaultStep, setStep}) => {

  const [customStep, setCustomStep] = useState(defaultStep);
  const [error, setError] = useState(false);

  const handleApply = (step: number) => setStep(step || 1);
  const debouncedHandleApply = useCallback(debounce(handleApply, 700), []);

  const onChangeStep = (e: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    const value = +e.target.value;
    if (value > 0) {
      setCustomStep(value);
      debouncedHandleApply(value);
      setError(false);
    } else {
      setError(true);
    }
  };

  return <TextField
    label="Step value"
    type="number"
    size="small"
    variant="outlined"
    value={customStep}
    error={error}
    helperText={error ? "step is out of allowed range" : " "}
    onChange={onChangeStep}/>;
};

export default StepConfigurator;
