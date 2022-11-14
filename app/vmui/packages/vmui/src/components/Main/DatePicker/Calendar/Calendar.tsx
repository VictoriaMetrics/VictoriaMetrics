import React, { FC, useEffect, useState } from "preact/compat";
import dayjs, { Dayjs } from "dayjs";
import CalendarHeader from "./CalendarHeader/CalendarHeader";
import CalendarBody from "./CalendarBody/CalendarBody";
import YearsList from "./YearsList/YearsList";
import TimePicker from "../TImePicker/TimePicker";
import { DATE_TIME_FORMAT } from "../../../../constants/date";
import "./style.scss";

interface DatePickerProps {
  date: Date | Dayjs
  format?: string
  timepicker?: boolean,
  onChange: (date: string) => void
}

const Calendar: FC<DatePickerProps> = ({
  date,
  timepicker = false,
  format = DATE_TIME_FORMAT,
  onChange
}) => {
  const [displayYears, setDisplayYears] = useState(false);
  const [viewDate, setViewDate] = useState(dayjs(date));
  const [selectDate, setSelectDate] = useState(dayjs(date));
  const [handleFocusHour, setHandleFocusHour] = useState(0);

  const toggleDisplayYears = () => {
    setDisplayYears(prev => !prev);
  };

  const handleChangeViewDate = (date: Dayjs) => {
    setViewDate(date);
    setDisplayYears(false);
  };

  const handleChangeSelectDate = (date: Dayjs) => {
    setSelectDate(date);
    setHandleFocusHour(prev => prev + 1);
  };

  const handleChangeTime = (time: string) => {
    const [hour, minute, second] = time.split(":");
    setSelectDate(prev => prev.set("hour", +hour).set("minute", +minute).set("second", +second));
  };

  useEffect(() => {
    if (selectDate.format() === dayjs(date).format()) return;
    onChange(selectDate.format(format));
  }, [selectDate]);

  return (
    <div className="vm-calendar">
      <CalendarHeader
        viewDate={viewDate}
        onChangeViewDate={handleChangeViewDate}
        toggleDisplayYears={toggleDisplayYears}
        displayYears={displayYears}
      />
      {!displayYears && (
        <CalendarBody
          viewDate={viewDate}
          selectDate={selectDate}
          onChangeSelectDate={handleChangeSelectDate}
        />
      )}
      {displayYears && (
        <YearsList
          viewDate={viewDate}
          onChangeViewDate={handleChangeViewDate}
        />
      )}
      {timepicker && (
        <TimePicker
          selectDate={selectDate}
          handleFocusHour={handleFocusHour}
          onChangeTime={handleChangeTime}
        />
      )}
    </div>
  );
};

export default Calendar;
