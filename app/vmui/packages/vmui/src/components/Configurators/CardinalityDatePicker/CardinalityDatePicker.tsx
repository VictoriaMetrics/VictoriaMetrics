import React, { FC, useEffect, useMemo, useRef } from "preact/compat";
import dayjs from "dayjs";
import Button from "../../Main/Button/Button";
import { ArrowDownIcon, CalendarIcon } from "../../Main/Icons";
import Tooltip from "../../Main/Tooltip/Tooltip";
import { getAppModeEnable } from "../../../utils/app-mode";
import { DATE_FORMAT } from "../../../constants/date";
import DatePicker from "../../Main/DatePicker/DatePicker";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import { useSearchParams } from "react-router-dom";
import useSearchParamsFromObject from "../../../hooks/useSearchParamsFromObject";

const CardinalityDatePicker: FC = () => {
  const { isMobile } = useDeviceDetect();
  const appModeEnable = getAppModeEnable();
  const buttonRef = useRef<HTMLDivElement>(null);

  const [searchParams] = useSearchParams();
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();

  const date = searchParams.get("date") || dayjs().tz().format(DATE_FORMAT);

  const dateFormatted = useMemo(() => dayjs.tz(date).format(DATE_FORMAT), [date]);

  const handleChangeDate = (val: string) => {
    setSearchParamsFromKeys({ date: val });
  };

  useEffect(() => {
    handleChangeDate(date);
  }, []);

  return (
    <div>
      <div ref={buttonRef}>
        {isMobile ? (
          <div className="vm-mobile-option">
            <span className="vm-mobile-option__icon"><CalendarIcon/></span>
            <div className="vm-mobile-option-text">
              <span className="vm-mobile-option-text__label">Date control</span>
              <span className="vm-mobile-option-text__value">{dateFormatted}</span>
            </div>
            <span className="vm-mobile-option__arrow"><ArrowDownIcon/></span>
          </div>
        ) : (
          <Tooltip title="Date control">
            <Button
              className={appModeEnable ? "" : "vm-header-button"}
              variant="contained"
              color="primary"
              startIcon={<CalendarIcon/>}
            >
              {dateFormatted}
            </Button>
          </Tooltip>
        )}
      </div>
      <DatePicker
        label="Date control"
        date={date || ""}
        format={DATE_FORMAT}
        onChange={handleChangeDate}
        targetRef={buttonRef}
      />
    </div>
  );
};

export default CardinalityDatePicker;
