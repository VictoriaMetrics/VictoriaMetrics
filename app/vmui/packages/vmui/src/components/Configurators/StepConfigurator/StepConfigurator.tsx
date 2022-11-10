import React, { FC, useCallback, useState } from "preact/compat";
import { ChangeEvent, useEffect } from "react";
import debounce from "lodash.debounce";
import { RestartIcon } from "../../Main/Icons";
import TextField from "../../Main/TextField/TextField";
import Button from "../../Main/Button/Button";

interface StepConfiguratorProps {
  defaultStep?: number,
  setStep: (step: number) => void,
}

const StepConfigurator: FC<StepConfiguratorProps> = ({ defaultStep, setStep }) => {

  const [customStep, setCustomStep] = useState(defaultStep);
  const [error, setError] = useState(false);

  const handleApply = (step: number) => setStep(step || 1);
  const debouncedHandleApply = useCallback(debounce(handleApply, 700), []);

  const onChangeStep = (val: string) => {
    const value = +val;
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

  return (
    <TextField
      label="Step value"
      type="number"
      value={customStep}
      error={error ? "step is out of allowed range" : ""}
      onChange={onChangeStep}
      endIcon={(
        <Button
          variant={"text"}
          size={"small"}
          startIcon={<RestartIcon/>}
          onClick={() => handleSetStep(defaultStep || 1)}
        />
      )}
    />
  );
};

export default StepConfigurator;
