import React, { Ref, useEffect, useMemo, useState, forwardRef } from "preact/compat";
import Calendar from "../../Main/DatePicker/Calendar/Calendar";
import dayjs, { Dayjs } from "dayjs";
import Popper from "../../Main/Popper/Popper";
import { DATE_TIME_FORMAT } from "../../../constants/date";

interface DatePickerProps {
  date: string | Date | Dayjs,
  targetRef: Ref<HTMLElement>
  format?: string
  timepicker?: boolean
  onChange: (val: string) => void
}

const DatePicker = forwardRef<HTMLDivElement, DatePickerProps>(({
  date,
  targetRef,
  format = DATE_TIME_FORMAT,
  timepicker,
  onChange,
}, ref) => {
  const [openCalendar, setOpenCalendar] = useState(false);
  const dateDayjs = useMemo(() => date ? dayjs(date) : dayjs(), [date]);

  const toggleOpenCalendar = () => {
    setOpenCalendar(prev => !prev);
  };

  const handleCloseCalendar = () => {
    setOpenCalendar(false);
  };

  const handleChangeDate = (val: string) => {
    if (!timepicker) handleCloseCalendar();
    onChange(val);
  };

  const handleKeyUp = (e: KeyboardEvent) => {
    if (e.key === "Escape" || e.key === "Enter") handleCloseCalendar();
  };

  useEffect(() => {
    targetRef.current?.addEventListener("click", toggleOpenCalendar);

    return () => {
      targetRef.current?.removeEventListener("click", toggleOpenCalendar);
    };
  }, [targetRef]);

  useEffect(() => {
    window.addEventListener("keyup", handleKeyUp);

    return () => {
      window.removeEventListener("keyup", handleKeyUp);
    };
  }, []);

  return (<>
    <Popper
      open={openCalendar}
      buttonRef={targetRef}
      placement="bottom-right"
      onClose={handleCloseCalendar}
    >
      <div ref={ref}>
        <Calendar
          date={dateDayjs}
          format={format}
          timepicker={timepicker}
          onChange={handleChangeDate}
          onClose={handleCloseCalendar}
        />
      </div>
    </Popper>
  </>);
});

export default DatePicker;
