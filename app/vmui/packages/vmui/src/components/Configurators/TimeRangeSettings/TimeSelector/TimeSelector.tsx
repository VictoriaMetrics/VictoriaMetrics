import React, { FC, useEffect, useState, useMemo, useRef } from "preact/compat";
import { dateFromSeconds, formatDateForNativeInput } from "../../../../utils/time";
import TimeDurationSelector from "../TimeDurationSelector/TimeDurationSelector";
import dayjs from "dayjs";
import { getAppModeEnable } from "../../../../utils/app-mode";
import { useTimeDispatch, useTimeState } from "../../../../state/time/TimeStateContext";
import { AlarmIcon, CalendarIcon, ClockIcon } from "../../../Main/Icons";
import Button from "../../../Main/Button/Button";
import Popper from "../../../Main/Popper/Popper";
import "./style.scss";
import Tooltip from "../../../Main/Tooltip/Tooltip";
import { DATE_TIME_FORMAT } from "../../../../constants/date";
import useResize from "../../../../hooks/useResize";
import DatePicker from "../../../Main/DatePicker/DatePicker";

export const TimeSelector: FC = () => {
  const documentSize = useResize(document.body);
  const displayFullDate = useMemo(() => documentSize.width > 1120, [documentSize]);

  const [until, setUntil] = useState<string>();
  const [from, setFrom] = useState<string>();

  const formFormat = useMemo(() => dayjs(from).format(DATE_TIME_FORMAT), [from]);
  const untilFormat = useMemo(() => dayjs(until).format(DATE_TIME_FORMAT), [until]);

  const { period: { end, start }, relativeTime } = useTimeState();
  const dispatch = useTimeDispatch();
  const appModeEnable = getAppModeEnable();

  useEffect(() => {
    setUntil(formatDateForNativeInput(dateFromSeconds(end)));
  }, [end]);

  useEffect(() => {
    setFrom(formatDateForNativeInput(dateFromSeconds(start)));
  }, [start]);

  const setDuration = ({ duration, until, id }: {duration: string, until: Date, id: string}) => {
    dispatch({ type: "SET_RELATIVE_TIME", payload: { duration, until, id } });
    setOpenOptions(false);
  };

  const formatRange = useMemo(() => {
    const startFormat = dayjs(dateFromSeconds(start)).format(DATE_TIME_FORMAT);
    const endFormat = dayjs(dateFromSeconds(end)).format(DATE_TIME_FORMAT);
    return {
      start: startFormat,
      end: endFormat
    };
  }, [start, end]);

  const dateTitle = useMemo(() => {
    const isRelativeTime = relativeTime && relativeTime !== "none";
    return isRelativeTime ? relativeTime.replace(/_/g, " ") : `${formatRange.start} - ${formatRange.end}`;
  }, [relativeTime, formatRange]);

  const fromRef = useRef<HTMLDivElement>(null);
  const untilRef = useRef<HTMLDivElement>(null);
  const [openOptions, setOpenOptions] = useState(false);
  const buttonRef = useRef<HTMLDivElement>(null);

  const setTimeAndClosePicker = () => {
    if (from && until) {
      dispatch({ type: "SET_PERIOD", payload: { from: new Date(from), to: new Date(until) } });
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
    >
      <div className="vm-time-selector">
        <div className="vm-time-selector-left">
          <div className="vm-time-selector-left-inputs">
            <div
              className="vm-time-selector-left-inputs__date"
              ref={fromRef}
            >
              <label>From:</label>
              <span>{formFormat}</span>
              <CalendarIcon/>
            </div>
            <DatePicker
              date={from || ""}
              onChange={handleFromChange}
              targetRef={fromRef}
              timepicker={true}
            />
            <div
              className="vm-time-selector-left-inputs__date"
              ref={untilRef}
            >
              <label>To:</label>
              <span>{untilFormat}</span>
              <CalendarIcon/>
            </div>
            <DatePicker
              date={until || ""}
              onChange={handleUntilChange}
              targetRef={untilRef}
              timepicker={true}
            />
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
