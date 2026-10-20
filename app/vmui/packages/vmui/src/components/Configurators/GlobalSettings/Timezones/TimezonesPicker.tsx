import { FC, useMemo, useRef } from "preact/compat";
import { getUTCByTimezone } from "../../../../utils/time";
import { ArrowDropDownIcon } from "../../../Main/Icons";
import classNames from "classnames";
import { Timezone } from "../../../../types";
import "./style.scss";
import useBoolean from "../../../../hooks/useBoolean";
import { useTimeDispatch, useTimeState } from "../../../../state/time/TimeStateContext";
import TimezonesList from "./TimezonesList";
import Popper from "../../../Main/Popper/Popper";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";

const TimezonesPicker: FC = () => {
  const { isMobile } = useDeviceDetect();
  const { timezone: stateTimezone } = useTimeState();
  const timeDispatch = useTimeDispatch();

  const triggerRef = useRef<HTMLDivElement>(null);

  const {
    value: isOpenList,
    toggle: toggleOpenList,
    setFalse: handleCloseList,
  } = useBoolean(false);

  const activeTimezone = useMemo(() => ({
    region: stateTimezone,
    utc: getUTCByTimezone(stateTimezone)
  }), [stateTimezone]);

  const handleSetTimezone = (tz: Timezone) => {
    timeDispatch({ type: "SET_TIMEZONE", payload: tz.region });
    handleCloseList();
  };

  return (
    <div className="vm-timezones">
      <div className="vm-server-configurator__title">
        Time zone
      </div>
      <div
        className="vm-timezones-item vm-timezones-item_selected"
        onClick={toggleOpenList}
        ref={triggerRef}
      >
        <div className="vm-timezones-item__title">{activeTimezone.region}</div>
        <div className="vm-timezones-item__utc">{activeTimezone.utc}</div>
        <div
          className={classNames({
            "vm-timezones-item__icon": true,
            "vm-timezones-item__icon_open": isOpenList
          })}
        >
          <ArrowDropDownIcon/>
        </div>
      </div>
      <Popper
        open={isOpenList}
        buttonRef={triggerRef}
        placement="bottom-left"
        onClose={handleCloseList}
        fullWidth
        title={isMobile ? "Time zone" : undefined}
      >
        <TimezonesList onChange={handleSetTimezone}/>
      </Popper>
    </div>
  );
};

export default TimezonesPicker;
