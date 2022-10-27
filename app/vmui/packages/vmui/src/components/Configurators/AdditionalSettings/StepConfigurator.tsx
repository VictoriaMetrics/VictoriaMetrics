import React, { FC, useCallback, useState } from "preact/compat";
import { ChangeEvent, useEffect } from "react";
import TextField from "@mui/material/TextField";
import debounce from "lodash.debounce";
import InputAdornment from "@mui/material/InputAdornment";
import Tooltip from "@mui/material/Tooltip";
import RestartAltIcon from "@mui/icons-material/RestartAlt";
import IconButton from "@mui/material/IconButton";

interface StepConfiguratorProps {
  defaultStep?: number,
  setStep: (step: number) => void,
}

const StepConfigurator: FC<StepConfiguratorProps> = ({ defaultStep, setStep }) => {

  const [customStep, setCustomStep] = useState(defaultStep);
  const [error, setError] = useState(false);

  const handleApply = (step: number) => setStep(step || 1);
  const debouncedHandleApply = useCallback(debounce(handleApply, 700), []);

  const onChangeStep = (e: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    const value = +e.target.value;
    if (!value) return;
    handleSetStep(value);
  };

  const handleSetStep = (value: number) => {
    if (value > 0) {
      setCustomStep(value);
      debouncedHandleApply(value);
      setError(false);
    } else {
      setError(true);
    }
  };

  useEffect(() => {
    if (defaultStep) handleSetStep(defaultStep);
  }, [defaultStep]);

  return <TextField
    label="Step value"
    type="number"
    size="small"
    variant="outlined"
    value={customStep}
    error={error}
    helperText={error ? "step is out of allowed range" : " "}
    onChange={onChangeStep}
    InputProps={{
      inputProps: { min: 0 },
      endAdornment: (
        <InputAdornment
          position="start"
          sx={{ mr: -0.5, cursor: "pointer" }}
        >
          <Tooltip title={"Reset step to default"}>
            <IconButton
              size={"small"}
              onClick={() => handleSetStep(defaultStep || 1)}
            >
              <RestartAltIcon fontSize={"small"} />
            </IconButton>
          </Tooltip>
        </InputAdornment>
      ),
    }}
  />;
};

export default StepConfigurator;
