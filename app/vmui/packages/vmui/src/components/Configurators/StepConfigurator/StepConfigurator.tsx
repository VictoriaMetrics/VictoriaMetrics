import { useStep } from "../../../state/step/StepStateContext";
import { FC, useEffect, useRef, useState } from "preact/compat";
import { ArrowDownIcon, RestartIcon, TimelineIcon } from "../../Main/Icons";
import TextField from "../../Main/TextField/TextField";
import Button from "../../Main/Button/Button";
import Tooltip from "../../Main/Tooltip/Tooltip";
import { ErrorTypes } from "../../../types";
import { supportedDurations } from "../../../utils/time";
import "./style.scss";
import { getAppModeEnable } from "../../../utils/app-mode";
import Popper from "../../Main/Popper/Popper";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import classNames from "classnames";
import useBoolean from "../../../hooks/useBoolean";
import Hyperlink from "../../Main/Hyperlink/Hyperlink";

const StepConfigurator: FC = () => {
  const appModeEnable = getAppModeEnable();
  const { isMobile } = useDeviceDetect();

  const { calculated, override, effective, setOverride, reset } = useStep();
  const [customStep, setCustomStep] = useState(effective);
  const [error, setError] = useState("");
  const edited = useRef(false);

  const {
    value: openOptions,
    toggle: toggleOpenOptions,
    setFalse: setCloseOptions,
  } = useBoolean(false);

  const buttonRef = useRef<HTMLDivElement>(null);

  const handleApply = () => {
    if (!edited.current || error) return;
    const normalized = (customStep.match(/([0-9]*\.[0-9]+|[0-9]+)([a-zA-Z]+)?/g) || []).join("");
    const durations = normalized.match(/[a-zA-Z]+/g) || [];
    const value = durations.length ? normalized : `${normalized}s`;
    setOverride(value);
    setCustomStep(value);
    edited.current = false;
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
    const normalized = value.replace(/,/g, ".").replace(/[^0-9.a-zA-Z]/g, "");
    const numbers = normalized.match(/[-+]?([0-9]*\.[0-9]+|[0-9]+)/g) || [];
    const durations = normalized.match(/[a-zA-Z]+/g) || [];
    const isValidNumbers = numbers.length && numbers.every(num => parseFloat(num) > 0);
    const isValidDuration = durations.every(d => supportedDurations.find(dur => dur.short === d));
    const isValidStep = isValidNumbers && isValidDuration;

    setCustomStep(normalized);
    edited.current = true;

    if (isValidStep) {
      setError("");
    } else {
      setError(ErrorTypes.validStep);
    }
  };

  const handleReset = () => {
    reset();
    setCustomStep(calculated);
    setError("");
    edited.current = false;
  };

  useEffect(() => {
    setCustomStep(effective);
    setError("");
    edited.current = false;
  }, [effective, override]);

  const textValue = override === null ? `auto (${effective})` : effective;

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
            <span className="vm-mobile-option-text__value">{textValue}</span>
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
          Step: {textValue}
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
              <Tooltip title={`Reset to auto step (${calculated})`}>
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
                href="https://prometheus.io/docs/prometheus/latest/querying/basics/#float-literals-and-time-durations"
                text="interval"
              /> between datapoints, which must be returned from the range query.
              The <code>query</code> is executed
              at <code>start</code>, <code>start+step</code>, <code>start+2*step</code>, …, <code>end</code> timestamps.
            </p>
            <p>
              Read more about <Hyperlink
                href="https://docs.victoriametrics.com/victoriametrics/keyconcepts/#range-query"
                text="Range"
              /> and <Hyperlink
                href="https://docs.victoriametrics.com/victoriametrics/keyconcepts/#instant-query"
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
