import { FC, useEffect, useRef } from "preact/compat";
import Button from "../../../../components/Main/Button/Button";
import classNames from "classnames";
import { ArrowDownIcon } from "../../../../components/Main/Icons";
import Tooltip from "../../../../components/Main/Tooltip/Tooltip";
import useBoolean from "../../../../hooks/useBoolean";
import { REFRESH_OPTIONS } from "./constants";
import Popper from "../../../../components/Main/Popper/Popper";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";
import { useAutoRefreshInterval } from "./hooks/useAutoRefreshInterval";
import { durationToMs, isValidDelay } from "./utils";
import { useTimeDispatch } from "../../../../state/time/TimeStateContext";
import "./style.scss";

const AutoRefreshControl: FC = () => {
  const { isMobile } = useDeviceDetect();
  const dispatch = useTimeDispatch();
  const { refreshInterval, setRefreshInterval } = useAutoRefreshInterval();

  const buttonRef = useRef<HTMLDivElement>(null);

  const {
    value: openOptions,
    toggle: toggleOpenOptions,
    setFalse: handleCloseOptions,
  } = useBoolean(false);

  const handleChange = (dur: string) => () => {
    setRefreshInterval(dur);
    handleCloseOptions();
  };

  useEffect(() => {
    const ms = durationToMs(refreshInterval);
    if (!isValidDelay(ms)) return;

    const timer = setInterval(() => {
      dispatch({ type: "RUN_QUERY" });
    }, ms);

    return () => clearInterval(timer);
  }, [refreshInterval, dispatch]);

  return (
    <>
      <Tooltip title="Auto-refresh control">
        <div ref={buttonRef}>
          <Button
            className="vm-auto-refresh-control-button"
            variant="contained"
            color="primary"
            fullWidth
            ariaLabel={`Auto-refresh control, current interval: ${refreshInterval}`}
            endIcon={(
              <div
                className={classNames({
                  "vm-auto-refresh-control-button__arrow": true,
                  "vm-auto-refresh-control-button__arrow_open": openOptions,
                })}
              >
                <ArrowDownIcon/>
              </div>
            )}
            onClick={toggleOpenOptions}
          />
        </div>
      </Tooltip>
      <Popper
        open={openOptions}
        placement="bottom-right"
        onClose={handleCloseOptions}
        buttonRef={buttonRef}
        title={isMobile ? "Auto-refresh duration" : undefined}
      >
        <div
          className={classNames({
            "vm-list": true,
            "vm-auto-refresh-control-list": true,
            "vm-auto-refresh-control-list_mobile": isMobile,
          })}
        >
          {REFRESH_OPTIONS.map(d => (
            <button
              className={classNames({
                "vm-list-item": true,
                "vm-list-item_mobile": isMobile,
                "vm-list-item_active": d === refreshInterval
              })}
              key={d}
              onClick={handleChange(d)}
            >
              {d}
            </button>
          ))}
        </div>
      </Popper>
    </>
  );
};

export default AutoRefreshControl;
