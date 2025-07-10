import { FC } from "preact/compat";
import { Dayjs } from "dayjs";
import { ArrowDownIcon, ArrowDropDownIcon } from "../../../Icons";

interface CalendarHeaderProps {
  viewDate: Dayjs
  onChangeViewDate: (date: Dayjs) => void
  showArrowNav: boolean
  toggleDisplayYears: () => void
}

const CalendarHeader: FC<CalendarHeaderProps> = ({ viewDate, showArrowNav, onChangeViewDate, toggleDisplayYears }) => {

  const setPrevMonth = () => {
    onChangeViewDate(viewDate.subtract(1, "month"));
  };

  const setNextMonth = () => {
    onChangeViewDate(viewDate.add(1, "month"));
  };

  return (
    <div className="vm-calendar-header">
      <div
        className="vm-calendar-header-left"
        onClick={toggleDisplayYears}
      >
        <span className="vm-calendar-header-left__date">
          {viewDate.format("MMMM YYYY")}
        </span>
        <div className="vm-calendar-header-left__select-year">
          <ArrowDropDownIcon/>
        </div>
      </div>
      {showArrowNav && (
        <div className="vm-calendar-header-right">
          <div
            className="vm-calendar-header-right__prev"
            onClick={setPrevMonth}
          >
            <ArrowDownIcon/>
          </div>
          <div
            className="vm-calendar-header-right__next"
            onClick={setNextMonth}
          >
            <ArrowDownIcon/>
          </div>
        </div>
      )}
    </div>
  );
};

export default CalendarHeader;
