import { FC, useEffect, useState } from "preact/compat";
import dayjs, { Dayjs } from "dayjs";
import CalendarHeader from "./CalendarHeader/CalendarHeader";
import CalendarBody from "./CalendarBody/CalendarBody";
import YearsList from "./YearsList/YearsList";
import { DATE_FORMAT, DATE_TIME_FORMAT } from "../../../../constants/date";
import "./style.scss";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";
import classNames from "classnames";
import MonthsList from "./MonthsList/MonthsList";
import Button from "../../Button/Button";

interface DatePickerProps {
  date: Date | Dayjs
  format?: string
  onChange: (date: string) => void
}

enum CalendarTypeView {
  "days",
  "months",
  "years"
}

const Calendar: FC<DatePickerProps> = ({
  date,
  format = DATE_TIME_FORMAT,
  onChange,
}) => {
  const [viewType, setViewType] = useState<CalendarTypeView>(CalendarTypeView.days);
  const [viewDate, setViewDate] = useState(dayjs.tz(date));
  const [selectDate, setSelectDate] = useState(dayjs.tz(date));

  const today = dayjs.tz();
  const viewDateIsToday = today.format(DATE_FORMAT) === viewDate.format(DATE_FORMAT);
  const { isMobile } = useDeviceDetect();

  const toggleDisplayYears = () => {
    setViewType(prev => prev === CalendarTypeView.years ? CalendarTypeView.days : CalendarTypeView.years);
  };

  const handleChangeViewDate = (date: Dayjs) => {
    setViewDate(date);
    setViewType(prev => prev === CalendarTypeView.years ? CalendarTypeView.months : CalendarTypeView.days);
  };

  const handleChangeSelectDate = (date: Dayjs) => {
    setSelectDate(date);
  };

  const handleToday = () => {
    setViewDate(today);
  };

  useEffect(() => {
    if (selectDate.format() === dayjs.tz(date).format()) return;
    onChange(selectDate.format(format));
  }, [selectDate]);

  useEffect(() => {
    const value = dayjs.tz(date);
    setViewDate(value);
    setSelectDate(value);
  }, [date]);

  return (
    <div
      className={classNames({
        "vm-calendar": true,
        "vm-calendar_mobile": isMobile,
      })}
    >
      <CalendarHeader
        viewDate={viewDate}
        onChangeViewDate={handleChangeViewDate}
        toggleDisplayYears={toggleDisplayYears}
        showArrowNav={viewType === CalendarTypeView.days}
      />
      {viewType === CalendarTypeView.days && (
        <CalendarBody
          viewDate={viewDate}
          selectDate={selectDate}
          onChangeSelectDate={handleChangeSelectDate}
        />
      )}
      {viewType === CalendarTypeView.years && (
        <YearsList
          viewDate={viewDate}
          onChangeViewDate={handleChangeViewDate}
        />
      )}
      {viewType === CalendarTypeView.months && (
        <MonthsList
          selectDate={selectDate}
          viewDate={viewDate}
          onChangeViewDate={handleChangeViewDate}
        />
      )}
      {!viewDateIsToday && (viewType === CalendarTypeView.days) && (
        <div className="vm-calendar-footer">
          <Button
            variant="text"
            size="small"
            onClick={handleToday}
          >
              show today
          </Button>
        </div>
      )}
    </div>
  );
};

export default Calendar;
