import React, { FC, useEffect, useState } from "preact/compat";
import { RestartIcon } from "../../Main/Icons";
import TextField from "../../Main/TextField/TextField";
import Button from "../../Main/Button/Button";
import Tooltip from "../../Main/Tooltip/Tooltip";
import { ErrorTypes } from "../../../types";
import { supportedDurations } from "../../../utils/time";

interface StepConfiguratorProps {
  defaultStep?: string,
  value?: string,
  setStep: (step: string) => void,
}

const StepConfigurator: FC<StepConfiguratorProps> = ({ value, defaultStep, setStep }) => {

  const [customStep, setCustomStep] = useState(value || defaultStep);
  const [error, setError] = useState("");

  const handleApply = (value?: string) => {
    const step = value || customStep || defaultStep || "1s";
    const durations = step.match(/[a-zA-Z]+/g) || [];
    setStep(!durations.length ? `${step}s` : step);
  };

  const handleChangeStep = (value: string) => {
    const numbers = value.match(/[-+]?([0-9]*\.[0-9]+|[0-9]+)/g) || [];
    const durations = value.match(/[a-zA-Z]+/g) || [];
    const isValidNumbers = numbers.length && numbers.every(num => parseFloat(num) > 0);
    const isValidDuration = durations.every(d => supportedDurations.find(dur => dur.short === d));
    const isValidStep = isValidNumbers && isValidDuration;

    setCustomStep(value);

    if (isValidStep) {
      setError("");
    } else {
      setError(ErrorTypes.validStep);
    }
  };

  const handleReset = () => {
    const value = defaultStep || "1s";
    handleChangeStep(value);
    handleApply(value);
  };

  useEffect(() => {
    if (value) handleChangeStep(value);
  }, [value]);

  return (
    <TextField
      label="Step value"
      value={customStep}
      error={error}
      onChange={handleChangeStep}
      onEnter={handleApply}
      onBlur={handleApply}
      endIcon={(
        <Tooltip title="Reset step to default">
          <Button
            variant={"text"}
            size={"small"}
            startIcon={<RestartIcon/>}
            onClick={handleReset}
          />
        </Tooltip>
      )}
    />
  );
};

export default StepConfigurator;
