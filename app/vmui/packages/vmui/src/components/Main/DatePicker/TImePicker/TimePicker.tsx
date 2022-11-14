import React, { FC, useEffect, useRef, useState } from "preact/compat";
import { Dayjs } from "dayjs";
import { FormEvent, FocusEvent } from "react";
import { ClockIcon } from "../../Icons";

interface CalendarTimepickerProps {
  selectDate: Dayjs
  handleFocusHour: number
  onChangeTime: (time: string) => void
}

const TimePicker: FC<CalendarTimepickerProps>= ({ selectDate, handleFocusHour, onChangeTime }) => {

  const [hours, setHours] = useState(selectDate.format("HH"));
  const [minutes, setMinutes] = useState(selectDate.format("mm"));
  const [seconds, setSeconds] = useState(selectDate.format("ss"));

  useEffect(() => {
    setHours(selectDate.format("HH"));
    setMinutes(selectDate.format("mm"));
    setSeconds(selectDate.format("ss"));
  }, [selectDate]);

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
  };

  const handleFocusInput = (e: FocusEvent<HTMLInputElement>) => {
    e.target.select();
  };

  useEffect(() => {
    onChangeTime(`${hours}:${minutes}:${seconds}`);
  }, [hours, minutes, seconds]);

  useEffect(() => {
    hoursRef.current && hoursRef.current.focus();
  }, [handleFocusHour]);

  return (
    <div className="vm-calendar-time-picker">
      <div className="vm-calendar-time-picker__icon">
        <ClockIcon/>
      </div>
      <input
        className="vm-calendar-time-picker__input"
        value={hours}
        onChange={handleChangeHours}
        onFocus={handleFocusInput}
        ref={hoursRef}
        type="number"
        min={0}
        max={24}
      />
      <span>:</span>
      <input
        className="vm-calendar-time-picker__input"
        value={minutes}
        onChange={handleChangeMinutes}
        onFocus={handleFocusInput}
        ref={minutesRef}
        type="number"
        min={0}
        max={60}
      />
      <span>:</span>
      <input
        className="vm-calendar-time-picker__input"
        value={seconds}
        onChange={handleChangeSeconds}
        onFocus={handleFocusInput}
        ref={secondsRef}
        type="number"
        min={0}
        max={60}
      />
    </div>
  );
};

export default TimePicker;
