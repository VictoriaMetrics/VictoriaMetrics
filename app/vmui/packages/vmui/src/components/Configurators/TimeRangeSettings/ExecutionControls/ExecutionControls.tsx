import { FC, useEffect, useRef, useState } from "preact/compat";
import { useTimeDispatch } from "../../../../state/time/TimeStateContext";
import { getAppModeEnable } from "../../../../utils/app-mode";
import Button from "../../../Main/Button/Button";
import { ArrowDownIcon, RefreshIcon, RestartIcon } from "../../../Main/Icons";
import Popper from "../../../Main/Popper/Popper";
import "./style.scss";
import classNames from "classnames";
import Tooltip from "../../../Main/Tooltip/Tooltip";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";
import useBoolean from "../../../../hooks/useBoolean";

interface AutoRefreshOption {
  seconds: number
  title: string
}

const delayOptions: AutoRefreshOption[] = [
  { seconds: 0, title: "Off" },
  { seconds: 1, title: "1s" },
  { seconds: 2, title: "2s" },
  { seconds: 5, title: "5s" },
  { seconds: 10, title: "10s" },
  { seconds: 30, title: "30s" },
  { seconds: 60, title: "1m" },
  { seconds: 300, title: "5m" },
  { seconds: 900, title: "15m" },
  { seconds: 1800, title: "30m" },
  { seconds: 3600, title: "1h" },
  { seconds: 7200, title: "2h" }
];

export const ExecutionControls: FC = () => {
  const { isMobile } = useDeviceDetect();

  const dispatch = useTimeDispatch();
  const appModeEnable = getAppModeEnable();
  const [autoRefresh, setAutoRefresh] = useState(false);

  const [selectedDelay, setSelectedDelay] = useState<AutoRefreshOption>(delayOptions[0]);

  const {
    value: openOptions,
    toggle: toggleOpenOptions,
    setFalse: handleCloseOptions,
  } = useBoolean(false);
  const optionsButtonRef = useRef<HTMLDivElement>(null);

  const handleChange = (d: AutoRefreshOption) => {
    if ((autoRefresh && !d.seconds) || (!autoRefresh && d.seconds)) {
      setAutoRefresh(prev => !prev);
    }
    setSelectedDelay(d);
    handleCloseOptions();
  };

  const handleUpdate = () => {
    dispatch({ type: "RUN_QUERY" });
  };

  useEffect(() => {
    const delay = selectedDelay.seconds;
    let timer: number;
    if (autoRefresh) {
      timer = setInterval(() => {
        dispatch({ type: "RUN_QUERY" });
      }, delay * 1000) as unknown as number;
    } else {
      setSelectedDelay(delayOptions[0]);
    }
    return () => {
      timer && clearInterval(timer);
    };
  }, [selectedDelay, autoRefresh]);

  const createHandlerChange = (d: AutoRefreshOption) => () => {
    handleChange(d);
  };

  return <>
    <div className="vm-execution-controls">
      <div
        className={classNames({
          "vm-execution-controls-buttons": true,
          "vm-execution-controls-buttons_mobile": isMobile,
          "vm-header-button": !appModeEnable,
        })}
      >
        {!isMobile && (
          <Tooltip title="Refresh dashboard">
            <Button
              variant="contained"
              color="primary"
              onClick={handleUpdate}
              startIcon={<RefreshIcon/>}
              ariaLabel="refresh dashboard"
            />
          </Tooltip>
        )}
        {isMobile ? (
          <div
            className="vm-mobile-option"
            onClick={toggleOpenOptions}
          >
            <span className="vm-mobile-option__icon"><RestartIcon/></span>
            <div className="vm-mobile-option-text">
              <span className="vm-mobile-option-text__label">Auto-refresh</span>
              <span className="vm-mobile-option-text__value">{selectedDelay.title}</span>
            </div>
            <span className="vm-mobile-option__arrow"><ArrowDownIcon/></span>
          </div>
        ) : (
          <Tooltip title="Auto-refresh control">
            <div ref={optionsButtonRef}>
              <Button
                variant="contained"
                color="primary"
                fullWidth
                endIcon={(
                  <div
                    className={classNames({
                      "vm-execution-controls-buttons__arrow": true,
                      "vm-execution-controls-buttons__arrow_open": openOptions,
                    })}
                  >
                    <ArrowDownIcon/>
                  </div>
                )}
                onClick={toggleOpenOptions}
              >
                {selectedDelay.title}
              </Button>
            </div>
          </Tooltip>
        )}
      </div>
    </div>
    <Popper
      open={openOptions}
      placement="bottom-right"
      onClose={handleCloseOptions}
      buttonRef={optionsButtonRef}
      title={isMobile ? "Auto-refresh duration" : undefined}
    >
      <div
        className={classNames({
          "vm-execution-controls-list": true,
          "vm-execution-controls-list_mobile": isMobile,
        })}
      >
        {delayOptions.map(d => (
          <div
            className={classNames({
              "vm-list-item": true,
              "vm-list-item_mobile": isMobile,
              "vm-list-item_active": d.seconds === selectedDelay.seconds
            })}
            key={d.seconds}
            onClick={createHandlerChange(d)}
          >
            {d.title}
          </div>
        ))}
      </div>
    </Popper>
  </>;
};
