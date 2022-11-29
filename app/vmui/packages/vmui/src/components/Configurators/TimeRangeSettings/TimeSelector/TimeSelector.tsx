import React, { FC, useEffect, useState, useMemo, useRef } from "preact/compat";
import { dateFromSeconds, formatDateForNativeInput, getUTCByTimezone } from "../../../../utils/time";
import TimeDurationSelector from "../TimeDurationSelector/TimeDurationSelector";
import dayjs from "dayjs";
import { getAppModeEnable } from "../../../../utils/app-mode";
import { useTimeDispatch, useTimeState } from "../../../../state/time/TimeStateContext";
import { AlarmIcon, CalendarIcon, ClockIcon } from "../../../Main/Icons";
import Button from "../../../Main/Button/Button";
import Popper from "../../../Main/Popper/Popper";
import Tooltip from "../../../Main/Tooltip/Tooltip";
import { DATE_TIME_FORMAT } from "../../../../constants/date";
import useResize from "../../../../hooks/useResize";
import DatePicker from "../../../Main/DatePicker/DatePicker";
import "./style.scss";
import useClickOutside from "../../../../hooks/useClickOutside";

export const TimeSelector: FC = () => {
  const wrapperRef = useRef<HTMLDivElement>(null);
  const documentSize = useResize(document.body);
  const displayFullDate = useMemo(() => documentSize.width > 1120, [documentSize]);

  const [until, setUntil] = useState<string>();
  const [from, setFrom] = useState<string>();

  const formFormat = useMemo(() => dayjs.tz(from).format(DATE_TIME_FORMAT), [from]);
  const untilFormat = useMemo(() => dayjs.tz(until).format(DATE_TIME_FORMAT), [until]);

  const { period: { end, start }, relativeTime, timezone } = useTimeState();
  const dispatch = useTimeDispatch();
  const appModeEnable = getAppModeEnable();

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
    setOpenOptions(false);
  };

  const formatRange = useMemo(() => {
    const startFormat = dayjs.tz(dateFromSeconds(start)).format(DATE_TIME_FORMAT);
    const endFormat = dayjs.tz(dateFromSeconds(end)).format(DATE_TIME_FORMAT);
    return {
      start: startFormat,
      end: endFormat
    };
  }, [start, end, timezone]);

  const dateTitle = useMemo(() => {
    const isRelativeTime = relativeTime && relativeTime !== "none";
    return isRelativeTime ? relativeTime.replace(/_/g, " ") : `${formatRange.start} - ${formatRange.end}`;
  }, [relativeTime, formatRange]);

  const fromRef = useRef<HTMLDivElement>(null);
  const untilRef = useRef<HTMLDivElement>(null);
  const fromPickerRef = useRef<HTMLDivElement>(null);
  const untilPickerRef = useRef<HTMLDivElement>(null);
  const [openOptions, setOpenOptions] = useState(false);
  const buttonRef = useRef<HTMLDivElement>(null);

  const setTimeAndClosePicker = () => {
    if (from && until) {
      dispatch({ type: "SET_PERIOD", payload: {
        from: dayjs.tz(from).toDate(),
        to: dayjs.tz(until).toDate()
      } });
    }
    setOpenOptions(false);
  };
  const handleFromChange = (from: string) => setFrom(from);

  const handleUntilChange = (until: string) => setUntil(until);

  const onApplyClick = () => setTimeAndClosePicker();

  const onSwitchToNow = () => dispatch({ type: "RUN_QUERY_TO_NOW" });

  const onCancelClick = () => {
    setUntil(formatDateForNativeInput(dateFromSeconds(end)));
    setFrom(formatDateForNativeInput(dateFromSeconds(start)));
    setOpenOptions(false);
  };

  const toggleOpenOptions = () => {
    setOpenOptions(prev => !prev);
  };

  const handleCloseOptions = () => {
    setOpenOptions(false);
  };

  useClickOutside(wrapperRef, (e) => {
    const target = e.target as HTMLElement;
    const isFromButton = fromRef?.current && fromRef.current.contains(target);
    const isUntilButton = untilRef?.current && untilRef.current.contains(target);
    const isFromPicker = fromPickerRef?.current && fromPickerRef?.current?.contains(target);
    const isUntilPicker = untilPickerRef?.current && untilPickerRef?.current?.contains(target);
    if (isFromButton || isUntilButton || isFromPicker || isUntilPicker) return;
    handleCloseOptions();
  });

  return <>
    <div ref={buttonRef}>
      <Tooltip title="Time range controls">
        <Button
          className={appModeEnable ? "" : "vm-header-button"}
          variant="contained"
          color="primary"
          startIcon={<ClockIcon/>}
          onClick={toggleOpenOptions}
        >
          {displayFullDate && <span>{dateTitle}</span>}
        </Button>
      </Tooltip>
    </div>
    <Popper
      open={openOptions}
      buttonRef={buttonRef}
      placement="bottom-right"
      onClose={handleCloseOptions}
      clickOutside={false}
    >
      <div
        className="vm-time-selector"
        ref={wrapperRef}
      >
        <div className="vm-time-selector-left">
          <div className="vm-time-selector-left-inputs">
            <div
              className="vm-time-selector-left-inputs__date"
              ref={fromRef}
            >
              <label>From:</label>
              <span>{formFormat}</span>
              <CalendarIcon/>
              <DatePicker
                ref={fromPickerRef}
                date={from || ""}
                onChange={handleFromChange}
                targetRef={fromRef}
                timepicker={true}
              />
            </div>
            <div
              className="vm-time-selector-left-inputs__date"
              ref={untilRef}
            >
              <label>To:</label>
              <span>{untilFormat}</span>
              <CalendarIcon/>
              <DatePicker
                ref={untilPickerRef}
                date={until || ""}
                onChange={handleUntilChange}
                targetRef={untilRef}
                timepicker={true}
              />
            </div>
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
              onClick={onApplyClick}
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
