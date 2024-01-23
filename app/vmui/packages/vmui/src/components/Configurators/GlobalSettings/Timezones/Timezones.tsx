import React, { FC, useMemo, useRef, useState } from "preact/compat";
import { getTimezoneList, getUTCByTimezone } from "../../../../utils/time";
import { ArrowDropDownIcon } from "../../../Main/Icons";
import classNames from "classnames";
import Popper from "../../../Main/Popper/Popper";
import Accordion from "../../../Main/Accordion/Accordion";
import dayjs from "dayjs";
import TextField from "../../../Main/TextField/TextField";
import { Timezone } from "../../../../types";
import "./style.scss";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";
import useBoolean from "../../../../hooks/useBoolean";

interface TimezonesProps {
  timezoneState: string;
  defaultTimezone?: string;
  onChange: (val: string) => void;
}

interface PinnedTimezone extends Timezone {
  title: string
}

const Timezones: FC<TimezonesProps> = ({ timezoneState, defaultTimezone, onChange }) => {
  const { isMobile } = useDeviceDetect();
  const timezones = getTimezoneList();

  const [search, setSearch] = useState("");
  const targetRef = useRef<HTMLDivElement>(null);

  const {
    value: openList,
    toggle: toggleOpenList,
    setFalse: handleCloseList,
  } = useBoolean(false);

  const pinnedTimezones = useMemo(() => [
    {
      title: `Default time (${defaultTimezone})`,
      region: defaultTimezone,
      utc: defaultTimezone ? getUTCByTimezone(defaultTimezone) : "UTC"
    },
    {
      title: `Browser Time (${dayjs.tz.guess()})`,
      region: dayjs.tz.guess(),
      utc: getUTCByTimezone(dayjs.tz.guess())
    },
    {
      title: "UTC (Coordinated Universal Time)",
      region: "UTC",
      utc: "UTC"
    },
  ].filter(t => t.region) as PinnedTimezone[], [defaultTimezone]);

  const searchTimezones = useMemo(() => {
    if (!search) return timezones;
    try {
      return getTimezoneList(search);
    } catch (e) {
      return {};
    }
  }, [search, timezones]);

  const timezonesGroups = useMemo(() => Object.keys(searchTimezones), [searchTimezones]);

  const activeTimezone = useMemo(() => ({
    region: timezoneState,
    utc: getUTCByTimezone(timezoneState)
  }), [timezoneState]);

  const handleChangeSearch = (val: string) => {
    setSearch(val);
  };

  const handleSetTimezone = (val: Timezone) => {
    onChange(val.region);
    setSearch("");
    handleCloseList();
  };

  const createHandlerSetTimezone = (val: Timezone) => () => {
    handleSetTimezone(val);
  };

  return (
    <div className="vm-timezones">
      <div className="vm-server-configurator__title">
        Time zone
      </div>
      <div
        className="vm-timezones-item vm-timezones-item_selected"
        onClick={toggleOpenList}
        ref={targetRef}
      >
        <div className="vm-timezones-item__title">{activeTimezone.region}</div>
        <div className="vm-timezones-item__utc">{activeTimezone.utc}</div>
        <div
          className={classNames({
            "vm-timezones-item__icon": true,
            "vm-timezones-item__icon_open": openList
          })}
        >
          <ArrowDropDownIcon/>
        </div>
      </div>
      <Popper
        open={openList}
        buttonRef={targetRef}
        placement="bottom-left"
        onClose={handleCloseList}
        fullWidth
        title={isMobile ? "Time zone" : undefined}
      >
        <div
          className={classNames({
            "vm-timezones-list": true,
            "vm-timezones-list_mobile": isMobile,
          })}
        >
          <div className="vm-timezones-list-header">
            <div className="vm-timezones-list-header__search">
              <TextField
                autofocus
                label="Search"
                value={search}
                onChange={handleChangeSearch}
              />
            </div>
            {pinnedTimezones.map((t, i) => t && (
              <div
                key={`${i}_${t.region}`}
                className="vm-timezones-item vm-timezones-list-group-options__item"
                onClick={createHandlerSetTimezone(t)}
              >
                <div className="vm-timezones-item__title">{t.title}</div>
                <div className="vm-timezones-item__utc">{t.utc}</div>
              </div>
            ))}
          </div>
          {timezonesGroups.map(t => (
            <div
              className="vm-timezones-list-group"
              key={t}
            >
              <Accordion
                defaultExpanded={true}
                title={<div className="vm-timezones-list-group__title">{t}</div>}
              >
                <div className="vm-timezones-list-group-options">
                  {searchTimezones[t] && searchTimezones[t].map(item => (
                    <div
                      className="vm-timezones-item vm-timezones-list-group-options__item"
                      onClick={createHandlerSetTimezone(item)}
                      key={item.search}
                    >
                      <div className="vm-timezones-item__title">{item.region}</div>
                      <div className="vm-timezones-item__utc">{item.utc}</div>
                    </div>
                  ))}
                </div>
              </Accordion>
            </div>
          ))}
        </div>
      </Popper>
    </div>
  );
};

export default Timezones;
