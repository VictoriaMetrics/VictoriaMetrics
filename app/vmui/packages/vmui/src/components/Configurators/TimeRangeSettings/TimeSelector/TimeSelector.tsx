import React, { FC, useEffect, useState, useMemo, useRef } from "preact/compat";
import { KeyboardEvent } from "react";
import { dateFromSeconds, formatDateForNativeInput } from "../../../../utils/time";
import TimeDurationSelector from "../TimeDurationSelector/TimeDurationSelector";
import dayjs from "dayjs";
import { getAppModeEnable } from "../../../../utils/app-mode";
import { useTimeDispatch, useTimeState } from "../../../../state/time/TimeStateContext";
import { AlarmIcon, ClockIcon } from "../../../Main/Icons";
import Button from "../../../Main/Button/Button";
import Popper from "../../../Main/Popper/Popper";
import "./style.scss";
import Tooltip from "../../../Main/Tooltip/Tooltip";

const formatDate = "YYYY-MM-DD HH:mm:ss";

export const TimeSelector: FC = () => {

  const displayFullDate = window.innerWidth > 1120;

  const [until, setUntil] = useState<string>();
  const [from, setFrom] = useState<string>();

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
    const startFormat = dayjs(dateFromSeconds(start)).format(formatDate);
    const endFormat = dayjs(dateFromSeconds(end)).format(formatDate);
    return {
      start: startFormat,
      end: endFormat
    };
  }, [start, end]);

  const [openOptions, setOpenOptions] = useState(false);
  const buttonRef = useRef<HTMLDivElement>(null);
  const setTimeAndClosePicker = () => {
    if (from && until) {
      dispatch({ type: "SET_PERIOD", payload: { from: new Date(from), to: new Date(until) } });
    }
    setOpenOptions(false);
  };
  const onFromChange = (from: dayjs.Dayjs | null) => setFrom(from?.format(formatDate));
  const onUntilChange = (until: dayjs.Dayjs | null) => setUntil(until?.format(formatDate));
  const onApplyClick = () => setTimeAndClosePicker();
  const onSwitchToNow = () => dispatch({ type: "RUN_QUERY_TO_NOW" });
  const onCancelClick = () => {
    setUntil(formatDateForNativeInput(dateFromSeconds(end)));
    setFrom(formatDateForNativeInput(dateFromSeconds(start)));
    setOpenOptions(false);
  };
  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key === "Enter" || e.keyCode === 13) {
      setTimeAndClosePicker();
    }
  };

  return <>
    <div ref={buttonRef}>
      <Tooltip title="Time range controls">
        <Button
          className={appModeEnable ? "" : "vm-header-button"}
          variant="contained"
          color="primary"
          startIcon={<ClockIcon/>}
          onClick={() => setOpenOptions(prev => !prev)}
        >
          {displayFullDate && <span>
            {relativeTime && relativeTime !== "none"
              ? relativeTime.replace(/_/g, " ")
              : `${formatRange.start} - ${formatRange.end}`}
          </span>}
        </Button>
      </Tooltip>
    </div>
    <Popper
      open={openOptions}
      buttonRef={buttonRef}
      placement="bottom-right"
      onClose={() => setOpenOptions(false)}
    >
      <div className="vm-time-selector">
        <div className="vm-time-selector-left">
          <div className="vm-time-selector-left__inputs">
            {/*<DateTimePicker*/}
            {/*  label="From"*/}
            {/*  ampm={false}*/}
            {/*  value={from}*/}
            {/*  onChange={onFromChange}*/}
            {/*  onError={console.log}*/}
            {/*  inputFormat={formatDate}*/}
            {/*  mask="____-__-__ __:__:__"*/}
            {/*  renderInput={(params) => <TextField*/}
            {/*    {...params}*/}
            {/*    variant="standard"*/}
            {/*    onKeyDown={onKeyDown}*/}
            {/*  />}*/}
            {/*  maxDate={dayjs(until)}*/}
            {/*  PopperProps={{ disablePortal: true }}*/}
            {/*/>*/}
            {/*<DateTimePicker*/}
            {/*  label="To"*/}
            {/*  ampm={false}*/}
            {/*  value={until}*/}
            {/*  onChange={onUntilChange}*/}
            {/*  onError={console.log}*/}
            {/*  inputFormat={formatDate}*/}
            {/*  mask="____-__-__ __:__:__"*/}
            {/*  renderInput={(params) => <TextField*/}
            {/*    {...params}*/}
            {/*    variant="standard"*/}
            {/*    onKeyDown={onKeyDown}*/}
            {/*  />}*/}
            {/*  PopperProps={{ disablePortal: true }}*/}
            {/*/>*/}
          </div>
          <Button
            variant="outlined"
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
