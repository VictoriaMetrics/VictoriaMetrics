import { FC, useEffect, useMemo } from "preact/compat";
import dayjs, { Dayjs } from "dayjs";
import classNames from "classnames";

interface CalendarYearsProps {
  viewDate: Dayjs
  onChangeViewDate: (date: Dayjs) => void
}

const YearsList: FC<CalendarYearsProps> = ({ viewDate, onChangeViewDate }) => {

  const today = dayjs().format("YYYY");
  const currentYear = useMemo(() => viewDate.format("YYYY"), [viewDate]);
  const years: Dayjs[] = useMemo(() => {
    const displayYears = 18;
    const year = dayjs();
    const startYear = year.subtract(displayYears/2, "year");
    return new Array(displayYears).fill(startYear).map((d, i) => d.add(i, "year"));
  }, [viewDate]);

  useEffect(() => {
    const selectedEl = document.getElementById(`vm-calendar-year-${currentYear}`);
    if (!selectedEl) return;
    selectedEl.scrollIntoView({ block: "center" });
  }, []);

  const createHandlerClick = (year: Dayjs) => () => {
    onChangeViewDate(year);
  };

  return (
    <div className="vm-calendar-years">
      {years.map(y => (
        <div
          className={classNames({
            "vm-calendar-years__year": true,
            "vm-calendar-years__year_selected": y.format("YYYY") === currentYear,
            "vm-calendar-years__year_today": y.format("YYYY") === today
          })}
          id={`vm-calendar-year-${y.format("YYYY")}`}
          key={y.format("YYYY")}
          onClick={createHandlerClick(y)}
        >
          {y.format("YYYY")}
        </div>
      ))}
    </div>
  );
};

export default YearsList;
