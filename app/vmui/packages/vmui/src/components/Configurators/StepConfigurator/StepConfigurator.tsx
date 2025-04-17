import React, { FC, useEffect, useMemo, useRef, useState } from "preact/compat";
import { ArrowDownIcon, RestartIcon, TimelineIcon } from "../../Main/Icons";
import TextField from "../../Main/TextField/TextField";
import Button from "../../Main/Button/Button";
import Tooltip from "../../Main/Tooltip/Tooltip";
import { ErrorTypes } from "../../../types";
import { getStepFromDuration, supportedDurations } from "../../../utils/time";
import { useTimeState } from "../../../state/time/TimeStateContext";
import { useGraphDispatch, useGraphState } from "../../../state/graph/GraphStateContext";
import usePrevious from "../../../hooks/usePrevious";
import "./style.scss";
import { getAppModeEnable } from "../../../utils/app-mode";
import Popper from "../../Main/Popper/Popper";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import classNames from "classnames";
import useBoolean from "../../../hooks/useBoolean";
import { useCustomPanelState } from "../../../state/customPanel/CustomPanelStateContext";
import Hyperlink from "../../Main/Hyperlink/Hyperlink";

const StepConfigurator: FC = () => {
  const appModeEnable = getAppModeEnable();
  const { isMobile } = useDeviceDetect();

  const { customStep: value, isHistogram } = useGraphState();
  const { period: { step, end, start } } = useTimeState();
  const graphDispatch = useGraphDispatch();
  const { displayType } = useCustomPanelState();

  const prevDuration = usePrevious(end - start);

  const defaultStep = useMemo(() => {
    return getStepFromDuration(end - start, isHistogram, displayType);
  }, [end, start, isHistogram, displayType]);
  const prevDefaultStep = usePrevious(defaultStep);

  const [customStep, setCustomStep] = useState(value || defaultStep);
  const [error, setError] = useState("");

  const isAutoStep = value === defaultStep;

  const {
    value: openOptions,
    toggle: toggleOpenOptions,
    setFalse: setCloseOptions,
  } = useBoolean(false);

  const buttonRef = useRef<HTMLDivElement>(null);

  const handleApply = (value?: string) => {
    const step = value || customStep || defaultStep || "1s";
    const durations = step.match(/[a-zA-Z]+/g) || [];
    const stepDur = !durations.length ? `${step}s` : step;
    graphDispatch({ type: "SET_CUSTOM_STEP", payload: stepDur });
    setCustomStep(stepDur);
    setError("");
  };

  const handleCloseOptions = () => {
    handleApply();
    setCloseOptions();
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
    if (value) {
      handleApply(value);
    }
  }, [value]);

  useEffect(() => {
    if (!value && defaultStep) {
      handleApply(defaultStep);
    }
  }, [defaultStep]);

  useEffect(() => {
    const dur = end - start;
    if (dur === prevDuration || !prevDuration || value !== prevDefaultStep) return;
    if (defaultStep) {
      handleApply(defaultStep);
    }
  }, [prevDuration, defaultStep]);

  useEffect(() => {
    if (step === value || step === defaultStep) handleApply(defaultStep);
  }, [isHistogram, displayType]);

  return (
    <div
      className="vm-step-control"
      ref={buttonRef}
    >
      {isMobile ? (
        <div
          className="vm-mobile-option"
          onClick={toggleOpenOptions}
        >
          <span className="vm-mobile-option__icon"><TimelineIcon/></span>
          <div className="vm-mobile-option-text">
            <span className="vm-mobile-option-text__label">Step</span>
            <span className="vm-mobile-option-text__value">{customStep}</span>
          </div>
          <span className="vm-mobile-option__arrow"><ArrowDownIcon/></span>
        </div>
      ) : (
        <Button
          className={appModeEnable ? "" : "vm-header-button"}
          variant="contained"
          color="primary"
          startIcon={<TimelineIcon/>}
          onClick={toggleOpenOptions}
        >
            Step: {isAutoStep ? `auto (${customStep})` : customStep}
        </Button>
      )}
      <Popper
        open={openOptions}
        placement="bottom-right"
        onClose={handleCloseOptions}
        buttonRef={buttonRef}
        title={isMobile ? "Query resolution step width" : undefined}
      >
        <div
          className={classNames({
            "vm-step-control-popper": true,
            "vm-step-control-popper_mobile": isMobile,
          })}
        >
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
              <Tooltip title={`Reset to auto step (${defaultStep})`}>
                <Button
                  size="small"
                  variant="text"
                  color="primary"
                  startIcon={<RestartIcon/>}
                  onClick={handleReset}
                  ariaLabel="reset step"
                />
              </Tooltip>
            )}
          />
          <div className="vm-step-control-popper-info">
            <p>
              <code>step</code> - the <Hyperlink
                href="https://prometheus.io/docs/prometheus/latest/querying/basics/#time-durations"
                text="interval"
              /> between datapoints, which must be returned from the range query.
              The <code>query</code> is executed
              at <code>start</code>, <code>start+step</code>, <code>start+2*step</code>, …, <code>end</code> timestamps.
            </p>
            <p>
              Read more about <Hyperlink
                href="https://docs.victoriametrics.com/keyconcepts/#range-query"
                text="Range"
              /> and <Hyperlink
                href="https://docs.victoriametrics.com/keyconcepts/#instant-query"
                text="Instant"
              /> queries.
            </p>
          </div>
        </div>
      </Popper>

    </div>
  );
};

export default StepConfigurator;
