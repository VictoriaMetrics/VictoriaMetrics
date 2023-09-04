import React, { FC, useEffect, useMemo, useRef, useState } from "preact/compat";
import { Dayjs } from "dayjs";
import { FormEvent, FocusEvent } from "react";
import classNames from "classnames";
import { useAppState } from "../../../../state/common/StateContext";

interface CalendarTimepickerProps {
  selectDate: Dayjs
  onChangeTime: (time: string) => void,
  onClose: () => void
}

enum TimeUnits { hour, minutes, seconds }


const TimePicker: FC<CalendarTimepickerProps>= ({ selectDate, onChangeTime, onClose }) => {
  const { isDarkTheme } = useAppState();

  const [activeField, setActiveField] = useState<TimeUnits>(TimeUnits.hour);
  const [hours, setHours] = useState(selectDate.format("HH"));
  const [minutes, setMinutes] = useState(selectDate.format("mm"));
  const [seconds, setSeconds] = useState(selectDate.format("ss"));

  const times = useMemo(() => {
    switch (activeField) {
      case TimeUnits.hour:
        return new Array(24).fill("00").map((h, i) => ({
          value: i,
          degrees: (i / 12) * 360,
          offset: i === 0 || i > 12,
          title: i ? `${i}` : h
        }));
      default:
        return new Array(60).fill("00").map((h, i) => ({
          value: i,
          degrees: (i / 60) * 360,
          offset: false,
          title: i ? `${i}` : h
        }));
    }
  }, [activeField, hours, minutes, seconds]);

  const arrowDegrees = useMemo(() => {
    switch (activeField) {
      case TimeUnits.hour:
        return (+hours / 12) * 360;
      case TimeUnits.minutes:
        return (+minutes / 60) * 360;
      case TimeUnits.seconds:
        return (+seconds / 60) * 360;
    }
  }, [activeField, hours, minutes, seconds]);

  const hoursRef = useRef<HTMLInputElement>(null);
  const minutesRef = useRef<HTMLInputElement>(null);
  const secondsRef = useRef<HTMLInputElement>(null);

  const handleChangeHours = (e: FormEvent<HTMLInputElement>) => {
    const el = e.target as HTMLInputElement;
    const value = el.value;
    const validValue = +value > 23 ? "23" : value;
    el.value = validValue;
    setHours(validValue);
    if (value.length > 1 && minutesRef.current) {
      minutesRef.current.focus();
    }
  };

  const handleChangeMinutes = (e: FormEvent<HTMLInputElement>) => {
    const el = e.target as HTMLInputElement;
    const value = el.value;
    const validValue = +value > 59 ? "59" : value;
    el.value = validValue;
    setMinutes(validValue);
    if (value.length > 1 && secondsRef.current) {
      secondsRef.current.focus();
    }
  };

  const handleChangeSeconds = (e: FormEvent<HTMLInputElement>) => {
    const el = e.target as HTMLInputElement;
    const value = el.value;
    const validValue = +value > 59 ? "59" : value;
    el.value = validValue;
    setSeconds(validValue);
    if (value.length > 1 && secondsRef.current) {
      onClose();
    }
  };

  const handleFocusInput = (unit: TimeUnits, e: FocusEvent<HTMLInputElement>) => {
    e.target.select();
    setActiveField(unit);
  };

  const createHandlerFocusInput = (unit: TimeUnits) => (e: FocusEvent<HTMLInputElement>) => {
    handleFocusInput(unit, e);
  };

  const createHandlerClick = (value: number) => () => {
    const valString = String(value);
    switch (activeField) {
      case TimeUnits.hour:
        setHours(valString);
        minutesRef.current && minutesRef.current.focus();
        break;
      case TimeUnits.minutes:
        setMinutes(valString);
        secondsRef.current && secondsRef.current.focus();
        break;
      case TimeUnits.seconds:
        setSeconds(valString);
        onClose();
        break;
    }
  };

  useEffect(() => {
    onChangeTime(`${hours}:${minutes}:${seconds}`);
  }, [hours, minutes, seconds]);

  useEffect(() => {
    setHours(selectDate.format("HH"));
    setMinutes(selectDate.format("mm"));
    setSeconds(selectDate.format("ss"));
  }, [selectDate]);

  useEffect(() => {
    hoursRef.current && hoursRef.current.focus();
  }, []);

  return (
    <div className="vm-calendar-time-picker">
      <div className="vm-calendar-time-picker-clock">
        <div
          className={classNames({
            "vm-calendar-time-picker-clock__arrow": true,
            "vm-calendar-time-picker-clock__arrow_offset": activeField === TimeUnits.hour && (hours === "00" || +hours > 12)
          })}
          style={{ transform: `rotate(${arrowDegrees}deg)` }}
        />
        {times.map(t => (
          <div
            className={classNames({
              "vm-calendar-time-picker-clock__time": true,
              "vm-calendar-time-picker-clock__time_offset": t.offset,
              "vm-calendar-time-picker-clock__time_hide": times.length > 24 && t.value%5
            })}
            key={t.value}
            style={{ transform: `rotate(${t.degrees}deg)` }}
            onClick={createHandlerClick(t.value)}
          >
            <span style={{ transform: `rotate(-${t.degrees}deg)` }}>
              {t.title}
            </span>
          </div>
        ))}
      </div>
      <div
        className={classNames({
          "vm-calendar-time-picker-fields": true,
          "vm-calendar-time-picker-fields_dark": isDarkTheme
        })}
      >
        <input
          className="vm-calendar-time-picker-fields__input"
          value={hours}
          onChange={handleChangeHours}
          onFocus={createHandlerFocusInput(TimeUnits.hour)}
          ref={hoursRef}
          type="number"
          min={0}
          max={24}
        />
        <span>:</span>
        <input
          className="vm-calendar-time-picker-fields__input"
          value={minutes}
          onChange={handleChangeMinutes}
          onFocus={createHandlerFocusInput(TimeUnits.minutes)}
          ref={minutesRef}
          type="number"
          min={0}
          max={60}
        />
        <span>:</span>
        <input
          className="vm-calendar-time-picker-fields__input"
          value={seconds}
          onChange={handleChangeSeconds}
          onFocus={createHandlerFocusInput(TimeUnits.seconds)}
          ref={secondsRef}
          type="number"
          min={0}
          max={60}
        />
      </div>
    </div>
  );
};

export default TimePicker;
