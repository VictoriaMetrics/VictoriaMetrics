import React, { FC, useCallback, useEffect, useState } from "preact/compat";
import debounce from "lodash.debounce";
import { RestartIcon } from "../../Main/Icons";
import TextField from "../../Main/TextField/TextField";
import Button from "../../Main/Button/Button";
import Tooltip from "../../Main/Tooltip/Tooltip";

interface StepConfiguratorProps {
  defaultStep?: number,
  value?: number,
  setStep: (step: number) => void,
}

const StepConfigurator: FC<StepConfiguratorProps> = ({ value, defaultStep, setStep }) => {

  const [customStep, setCustomStep] = useState(value || defaultStep);
  const [error, setError] = useState("");

  const handleApply = (step: number) => setStep(step || 1);
  const debouncedHandleApply = useCallback(debounce(handleApply, 500), []);

  const onChangeStep = (val: string) => {
    const value = +val;
    if (!value) return;
    handleSetStep(value);
  };

  const handleSetStep = (value: number) => {
    if (value > 0) {
      setCustomStep(value);
      debouncedHandleApply(value);
      setError("");
    } else {
      setError("step is out of allowed range");
    }
  };

  const handleReset = () => {
    handleSetStep(defaultStep || 1);
  };

  useEffect(() => {
    if (value) handleSetStep(value);
  }, [value]);

  return (
    <TextField
      label="Step value of seconds"
      type="number"
      value={customStep}
      error={error}
      onChange={onChangeStep}
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
