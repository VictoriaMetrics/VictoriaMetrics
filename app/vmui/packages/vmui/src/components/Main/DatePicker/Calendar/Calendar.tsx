import React, { FC, useEffect, useState } from "preact/compat";
import dayjs, { Dayjs } from "dayjs";
import CalendarHeader from "./CalendarHeader/CalendarHeader";
import CalendarBody from "./CalendarBody/CalendarBody";
import YearsList from "./YearsList/YearsList";
import TimePicker from "../TImePicker/TimePicker";
import { DATE_TIME_FORMAT } from "../../../../constants/date";
import "./style.scss";
import { CalendarIcon, ClockIcon } from "../../Icons";
import Tabs from "../../Tabs/Tabs";

interface DatePickerProps {
  date: Date | Dayjs
  format?: string
  timepicker?: boolean,
  onChange: (date: string) => void
  onClose?: () => void
}

const tabs = [
  { value: "date", icon: <CalendarIcon/> },
  { value: "time", icon: <ClockIcon/> }
];

const Calendar: FC<DatePickerProps> = ({
  date,
  timepicker = false,
  format = DATE_TIME_FORMAT,
  onChange,
  onClose
}) => {
  const [displayYears, setDisplayYears] = useState(false);
  const [viewDate, setViewDate] = useState(dayjs.tz(date));
  const [selectDate, setSelectDate] = useState(dayjs.tz(date));
  const [tab, setTab] = useState(tabs[0].value);

  const toggleDisplayYears = () => {
    setDisplayYears(prev => !prev);
  };

  const handleChangeViewDate = (date: Dayjs) => {
    setViewDate(date);
    setDisplayYears(false);
  };

  const handleChangeSelectDate = (date: Dayjs) => {
    setSelectDate(date);
    if (timepicker) setTab("time");
  };

  const handleChangeTime = (time: string) => {
    const [hour, minute, second] = time.split(":");
    setSelectDate(prev => prev.set("hour", +hour).set("minute", +minute).set("second", +second));
  };

  const handleChangeTab = (value: string) => {
    setTab(value);
  };

  const handleClose = () => {
    onClose && onClose();
  };

  useEffect(() => {
    if (selectDate.format() === dayjs.tz(date).format()) return;
    onChange(selectDate.format(format));
  }, [selectDate]);

  return (
    <div className="vm-calendar">
      {tab === "date" && (
        <CalendarHeader
          viewDate={viewDate}
          onChangeViewDate={handleChangeViewDate}
          toggleDisplayYears={toggleDisplayYears}
          displayYears={displayYears}
        />
      )}

      {tab === "date" && (
        <>
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
        </>
      )}

      {tab === "time" && (
        <TimePicker
          selectDate={selectDate}
          onChangeTime={handleChangeTime}
          onClose={handleClose}
        />
      )}

      {timepicker && (
        <div className="vm-calendar__tabs">
          <Tabs
            activeItem={tab}
            items={tabs}
            onChange={handleChangeTab}
            indicatorPlacement="top"
          />
        </div>
      )}
    </div>
  );
};

export default Calendar;
