import React, { FC, useMemo, useRef } from "preact/compat";
import { useCardinalityState, useCardinalityDispatch } from "../../../state/cardinality/CardinalityStateContext";
import dayjs from "dayjs";
import Button from "../../Main/Button/Button";
import { CalendarIcon } from "../../Main/Icons";
import Tooltip from "../../Main/Tooltip/Tooltip";
import { getAppModeEnable } from "../../../utils/app-mode";
import { DATE_FORMAT } from "../../../constants/date";
import DatePicker from "../../Main/DatePicker/DatePicker";

const CardinalityDatePicker: FC = () => {
  const appModeEnable = getAppModeEnable();
  const buttonRef = useRef<HTMLDivElement>(null);

  const { date } = useCardinalityState();
  const cardinalityDispatch = useCardinalityDispatch();

  const dateFormatted = useMemo(() => dayjs.tz(date).format(DATE_FORMAT), [date]);

  const handleChangeDate = (val: string) => {
    cardinalityDispatch({ type: "SET_DATE", payload: val });
  };

  return (
    <div>
      <div ref={buttonRef}>
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
      </div>
      <DatePicker
        date={date || ""}
        format={DATE_FORMAT}
        onChange={handleChangeDate}
        targetRef={buttonRef}
      />
    </div>
  );
};

export default CardinalityDatePicker;
