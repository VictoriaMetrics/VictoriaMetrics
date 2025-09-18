import { FC, useEffect, useMemo } from "preact/compat";
import dayjs, { Dayjs } from "dayjs";
import classNames from "classnames";

interface CalendarMonthsProps {
  viewDate: Dayjs,
  selectDate: Dayjs

  onChangeViewDate: (date: Dayjs) => void
}

const MonthsList: FC<CalendarMonthsProps> = ({ viewDate, selectDate, onChangeViewDate }) => {

  const today = dayjs().format("MM");
  const currentMonths = useMemo(() => selectDate.format("MM"), [selectDate]);
  const months: Dayjs[] = useMemo(() => {
    return new Array(12).fill("").map((d, i) => dayjs(viewDate).month(i));
  }, [viewDate]);

  useEffect(() => {
    const selectedEl = document.getElementById(`vm-calendar-year-${currentMonths}`);
    if (!selectedEl) return;
    selectedEl.scrollIntoView({ block: "center" });
  }, []);

  const createHandlerClick = (date: Dayjs) => () => {
    onChangeViewDate(date);
  };

  return (
    <div className="vm-calendar-years">
      {months.map(m => (
        <div
          className={classNames({
            "vm-calendar-years__year": true,
            "vm-calendar-years__year_selected": m.format("MM") === currentMonths,
            "vm-calendar-years__year_today": m.format("MM") === today
          })}
          id={`vm-calendar-year-${m.format("MM")}`}
          key={m.format("MM")}
          onClick={createHandlerClick(m)}
        >
          {m.format("MMMM")}
        </div>
      ))}
    </div>
  );
};

export default MonthsList;
