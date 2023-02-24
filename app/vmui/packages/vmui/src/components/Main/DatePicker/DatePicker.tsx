import React, { Ref, useEffect, useMemo, useState, forwardRef } from "preact/compat";
import Calendar from "../../Main/DatePicker/Calendar/Calendar";
import dayjs, { Dayjs } from "dayjs";
import Popper from "../../Main/Popper/Popper";
import { DATE_TIME_FORMAT } from "../../../constants/date";
import useDeviceDetect from "../../../hooks/useDeviceDetect";

interface DatePickerProps {
  date: string | Date | Dayjs,
  targetRef: Ref<HTMLElement>
  format?: string
  timepicker?: boolean
  label?: string
  onChange: (val: string) => void
}

const DatePicker = forwardRef<HTMLDivElement, DatePickerProps>(({
  date,
  targetRef,
  format = DATE_TIME_FORMAT,
  timepicker,
  onChange,
  label
}, ref) => {
  const [openCalendar, setOpenCalendar] = useState(false);
  const dateDayjs = useMemo(() => date ? dayjs.tz(date) : dayjs().tz(), [date]);
  const { isMobile } = useDeviceDetect();

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
      title={isMobile ? label : undefined}
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
