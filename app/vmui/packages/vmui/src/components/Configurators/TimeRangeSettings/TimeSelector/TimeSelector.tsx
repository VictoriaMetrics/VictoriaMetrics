import { FC, useEffect, useState, useMemo, useRef } from "preact/compat";
import { dateFromSeconds, formatDateForNativeInput, getRelativeTime, getUTCByTimezone } from "../../../../utils/time";
import TimeDurationSelector from "../TimeDurationSelector/TimeDurationSelector";
import dayjs from "dayjs";
import { getAppModeEnable } from "../../../../utils/app-mode";
import { useTimeDispatch, useTimeState } from "../../../../state/time/TimeStateContext";
import { AlarmIcon, ArrowDownIcon, ClockIcon } from "../../../Main/Icons";
import Button from "../../../Main/Button/Button";
import Popper from "../../../Main/Popper/Popper";
import Tooltip from "../../../Main/Tooltip/Tooltip";
import { DATE_TIME_FORMAT } from "../../../../constants/date";
import "./style.scss";
import useClickOutside from "../../../../hooks/useClickOutside";
import classNames from "classnames";
import { useAppState } from "../../../../state/common/StateContext";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";
import DateTimeInput from "../../../Main/DatePicker/DateTimeInput/DateTimeInput";
import useBoolean from "../../../../hooks/useBoolean";
import useWindowSize from "../../../../hooks/useWindowSize";
import usePrevious from "../../../../hooks/usePrevious";

export const TimeSelector: FC = () => {
  const { isMobile } = useDeviceDetect();
  const { isDarkTheme } = useAppState();
  const wrapperRef = useRef<HTMLDivElement>(null);
  const documentSize = useWindowSize();
  const displayFullDate = useMemo(() => documentSize.width > 1120, [documentSize]);

  const [until, setUntil] = useState<string>();
  const [from, setFrom] = useState<string>();

  const { period: { end, start }, relativeTime, timezone, duration } = useTimeState();
  const dispatch = useTimeDispatch();
  const appModeEnable = getAppModeEnable();
  const prevTimezone = usePrevious(timezone);

  const {
    value: openOptions,
    toggle: toggleOpenOptions,
    setFalse: handleCloseOptions,
  } = useBoolean(false);

  const activeTimezone = useMemo(() => ({
    region: timezone,
    utc: getUTCByTimezone(timezone)
  }), [timezone]);

  useEffect(() => {
    setUntil(formatDateForNativeInput(dateFromSeconds(end)));
  }, [timezone, end]);

  useEffect(() => {
    setFrom(formatDateForNativeInput(dateFromSeconds(start)));
  }, [timezone, start]);

  const setDuration = ({ duration, until, id }: {duration: string, until: Date, id: string}) => {
    dispatch({ type: "SET_RELATIVE_TIME", payload: { duration, until, id } });
    handleCloseOptions();
  };

  const formatRange = useMemo(() => {
    const startFormat = dayjs.tz(dateFromSeconds(start)).format(DATE_TIME_FORMAT);
    const endFormat = dayjs.tz(dateFromSeconds(end)).format(DATE_TIME_FORMAT);
    return { start: startFormat, end: endFormat };
  }, [start, end, timezone]);

  const dateTitle = useMemo(() => {
    const isRelativeTime = relativeTime && relativeTime !== "none";
    return isRelativeTime ? relativeTime.replace(/_/g, " ") : `${formatRange.start} - ${formatRange.end}`;
  }, [relativeTime, formatRange]);

  const fromPickerRef = useRef<HTMLDivElement>(null);
  const untilPickerRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLDivElement>(null);

  const setTimeAndClosePicker = () => {
    if (from && until) {
      dispatch({ type: "SET_PERIOD", payload: {
        from: dayjs.tz(from).toDate(),
        to: dayjs.tz(until).toDate()
      } });
    }
    handleCloseOptions();
  };

  const onSwitchToNow = () => dispatch({ type: "RUN_QUERY_TO_NOW" });

  const onCancelClick = () => {
    setUntil(formatDateForNativeInput(dateFromSeconds(end)));
    setFrom(formatDateForNativeInput(dateFromSeconds(start)));
    handleCloseOptions();
  };

  useEffect(() => {
    const value = getRelativeTime({
      relativeTimeId: relativeTime,
      defaultDuration: duration,
      defaultEndInput: dateFromSeconds(end),
    });
    if (prevTimezone && timezone !== prevTimezone) {
      setDuration({ id: value.relativeTimeId, duration: value.duration, until: value.endInput });
    }
  }, [timezone, prevTimezone]);

  useClickOutside(wrapperRef, (e) => {
    if (isMobile) return;
    const target = e.target as HTMLElement;
    const isFromPicker = fromPickerRef?.current && fromPickerRef?.current?.contains(target);
    const isUntilPicker = untilPickerRef?.current && untilPickerRef?.current?.contains(target);
    if (isFromPicker || isUntilPicker) return;
    handleCloseOptions();
  });

  return <>
    <div ref={buttonRef}>
      {isMobile ? (
        <div
          className="vm-mobile-option"
          onClick={toggleOpenOptions}
        >
          <span className="vm-mobile-option__icon"><ClockIcon/></span>
          <div className="vm-mobile-option-text">
            <span className="vm-mobile-option-text__label">Time range</span>
            <span className="vm-mobile-option-text__value">{dateTitle}</span>
          </div>
          <span className="vm-mobile-option__arrow"><ArrowDownIcon/></span>
        </div>
      ) : (
        <Tooltip title={displayFullDate ? "Time range controls" : dateTitle}>
          <Button
            className={appModeEnable ? "" : "vm-header-button"}
            variant="contained"
            color="primary"
            startIcon={<ClockIcon/>}
            onClick={toggleOpenOptions}
            ariaLabel="time range controls"
          >
            {displayFullDate && <span>{dateTitle}</span>}
          </Button>
        </Tooltip>
      )}
    </div>
    <Popper
      open={openOptions}
      buttonRef={buttonRef}
      placement="bottom-right"
      onClose={handleCloseOptions}
      clickOutside={false}
      title={isMobile ? "Time range controls" : ""}
    >
      <div
        className={classNames({
          "vm-time-selector": true,
          "vm-time-selector_mobile": isMobile
        })}
        ref={wrapperRef}
      >
        <div className="vm-time-selector-left">
          <div
            className={classNames({
              "vm-time-selector-left-inputs": true,
              "vm-time-selector-left-inputs_dark": isDarkTheme
            })}
          >
            <DateTimeInput
              value={from}
              label="From:"
              pickerLabel="Date From"
              pickerRef={fromPickerRef}
              onChange={setFrom}
              onEnter={setTimeAndClosePicker}
            />
            <DateTimeInput
              value={until}
              label="To:"
              pickerLabel="Date To"
              pickerRef={untilPickerRef}
              onChange={setUntil}
              onEnter={setTimeAndClosePicker}
            />
          </div>
          <div className="vm-time-selector-left-timezone">
            <div className="vm-time-selector-left-timezone__title">{activeTimezone.region}</div>
            <div className="vm-time-selector-left-timezone__utc">{activeTimezone.utc}</div>
          </div>
          <Button
            variant="text"
            startIcon={<AlarmIcon />}
            onClick={onSwitchToNow}
          >
            switch to now
          </Button>
          <div className="vm-time-selector-left__controls">
            <Button
              color="error"
              variant="outlined"
              onClick={onCancelClick}
            >
              Cancel
            </Button>
            <Button
              color="primary"
              onClick={setTimeAndClosePicker}
            >
              Apply
            </Button>
          </div>
        </div>
        <TimeDurationSelector
          relativeTime={relativeTime || ""}
          setDuration={setDuration}
        />
      </div>
    </Popper>
  </>;
};
