import React, { FC, useEffect, useRef, useState } from "preact/compat";
import { RestartIcon, TimelineIcon } from "../../Main/Icons";
import TextField from "../../Main/TextField/TextField";
import Button from "../../Main/Button/Button";
import Tooltip from "../../Main/Tooltip/Tooltip";
import { ErrorTypes } from "../../../types";
import { supportedDurations } from "../../../utils/time";
import { useTimeState } from "../../../state/time/TimeStateContext";
import { useGraphDispatch, useGraphState } from "../../../state/graph/GraphStateContext";
import usePrevious from "../../../hooks/usePrevious";
import "./style.scss";
import { getAppModeEnable } from "../../../utils/app-mode";
import Popper from "../../Main/Popper/Popper";

const StepConfigurator: FC = () => {
  const appModeEnable = getAppModeEnable();

  const { customStep: value } = useGraphState();
  const { period: { step: defaultStep } } = useTimeState();
  const graphDispatch = useGraphDispatch();

  const { period: duration } = useTimeState();
  const prevDuration = usePrevious(duration);

  const [openOptions, setOpenOptions] = useState(false);
  const [customStep, setCustomStep] = useState(value || defaultStep);
  const [error, setError] = useState("");

  const buttonRef = useRef<HTMLDivElement>(null);

  const toggleOpenOptions = () => {
    setOpenOptions(prev => !prev);
  };

  const handleCloseOptions = () => {
    setOpenOptions(false);
  };

  const handleApply = (value?: string) => {
    const step = value || customStep || defaultStep || "1s";
    const durations = step.match(/[a-zA-Z]+/g) || [];
    const stepDur = !durations.length ? `${step}s` : step;
    graphDispatch({ type: "SET_CUSTOM_STEP", payload: stepDur });
    setCustomStep(stepDur);
    setError("");
  };

  const handleFocus = () => {
    if (document.activeElement instanceof HTMLInputElement) {
      document.activeElement.select();
    }
  };

  const handleEnter = () => {
    handleApply();
    handleCloseOptions();
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
    if (value) handleApply(value);
  }, [value]);

  useEffect(() => {
    if (!value && defaultStep) handleApply(defaultStep);
  }, [defaultStep]);

  useEffect(() => {
    if (duration === prevDuration || !prevDuration) return;
    if (defaultStep) handleApply(defaultStep);
  }, [duration, prevDuration, defaultStep]);

  return (
    <div
      className="vm-step-control"
      ref={buttonRef}
    >
      <Tooltip title="Query resolution step width">
        <Button
          className={appModeEnable ? "" : "vm-header-button"}
          variant="contained"
          color="primary"
          startIcon={<TimelineIcon/>}
          onClick={toggleOpenOptions}
        >
          STEP {customStep}
        </Button>
      </Tooltip>
      <Popper
        open={openOptions}
        placement="bottom-right"
        onClose={handleCloseOptions}
        buttonRef={buttonRef}
      >
        <div className="vm-step-control-popper">
          <TextField
            autofocus
            label="Step value"
            value={customStep}
            error={error}
            onChange={handleChangeStep}
            onEnter={handleEnter}
            onFocus={handleFocus}
            onBlur={handleApply}
            endIcon={(
              <Tooltip title={`Reset step to default value - ${defaultStep}`}>
                <Button
                  size="small"
                  variant="text"
                  color="primary"
                  startIcon={<RestartIcon/>}
                  onClick={handleReset}
                />
              </Tooltip>
            )}
          />
          <div className="vm-step-control-popper-info">
            <code>step</code> - the <a
              className="vm-link vm-link_colored"
              href="https://prometheus.io/docs/prometheus/latest/querying/basics/#time-durations"
              target="_blank"
              rel="noreferrer"
            >
            interval
            </a>
            between datapoints, which must be returned from the range query.
            The <code>query</code> is executed at
            <code>start</code>, <code>start+step</code>, <code>start+2*step</code>, …, <code>end</code> timestamps.
            <a
              className="vm-link vm-link_colored"
              href="https://docs.victoriametrics.com/keyConcepts.html#range-query"
              target="_blank"
              rel="noreferrer"
            >
              Read more about Range query
            </a>
          </div>
        </div>
      </Popper>

    </div>
  );
};

export default StepConfigurator;
