import { FC } from "preact/compat";
import { Dayjs } from "dayjs";
import { ArrowDownIcon, ArrowDropDownIcon } from "../../../Icons";
import classNames from "classnames";

interface CalendarHeaderProps {
  viewDate: Dayjs
  onChangeViewDate: (date: Dayjs) => void
  showArrowNav: boolean
  toggleDisplayYears: () => void
  hasNext: boolean
  hasPrev: boolean
}

const CalendarHeader: FC<CalendarHeaderProps> = ({ hasPrev, hasNext, viewDate, showArrowNav, onChangeViewDate, toggleDisplayYears }) => {

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
            className={classNames({
              "vm-calendar-header-right__prev": true,
              "vm-calendar-header-right_disabled": !hasPrev,
            })}
            onClick={hasPrev ? setPrevMonth : undefined}
          >
            <ArrowDownIcon/>
          </div>
          <div
            className={classNames({
              "vm-calendar-header-right__next": true,
              "vm-calendar-header-right_disabled": !hasNext,
            })}
            onClick={hasNext ? setNextMonth : undefined}
          >
            <ArrowDownIcon/>
          </div>
        </div>
      )}
    </div>
  );
};

export default CalendarHeader;
