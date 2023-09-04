import React, { FC, useMemo } from "preact/compat";
import dayjs, { Dayjs } from "dayjs";
import classNames from "classnames";

interface CalendarBodyProps {
  viewDate: Dayjs
  selectDate: Dayjs
  onChangeSelectDate: (date: Dayjs) => void
}

const weekday = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];

const CalendarBody: FC<CalendarBodyProps> = ({ viewDate, selectDate, onChangeSelectDate }) => {
  const format = "YYYY-MM-DD";
  const today = dayjs().tz().startOf("day");

  const days: (Dayjs|null)[] = useMemo(() => {
    const result = new Array(42).fill(null);
    const startDate = viewDate.startOf("month");
    const endDate = viewDate.endOf("month");
    const days = endDate.diff(startDate, "day") + 1;
    const monthDays = new Array(days).fill(startDate).map((d,i) => d.add(i, "day"));
    const startOfWeek = startDate.day();
    result.splice(startOfWeek, days, ...monthDays);
    return result;
  }, [viewDate]);

  const createHandlerSelectDate = (d: Dayjs | null) => () => {
    if (d) onChangeSelectDate(d);
  };

  return (
    <div className="vm-calendar-body">
      {weekday.map(w => (
        <div
          className="vm-calendar-body-cell vm-calendar-body-cell_weekday"
          key={w}
        >
          {w[0]}
        </div>
      ))}

      {days.map((d, i) => (
        <div
          className={classNames({
            "vm-calendar-body-cell": true,
            "vm-calendar-body-cell_day": true,
            "vm-calendar-body-cell_day_empty": !d,
            "vm-calendar-body-cell_day_active": (d && d.format(format)) === selectDate.format(format),
            "vm-calendar-body-cell_day_today": (d && d.format(format)) === today.format(format)
          })}
          key={d ? d.format(format) : i}
          onClick={createHandlerSelectDate(d)}
        >
          {d && d.format("D")}
        </div>
      ))}
    </div>
  );
};

export default CalendarBody;
