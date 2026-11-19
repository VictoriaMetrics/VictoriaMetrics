import { FC, useMemo, useState } from "preact/compat";
import { getBrowserTimezone, getTimezoneList, getUTCByTimezone } from "../../../../utils/time";
import classNames from "classnames";
import Accordion from "../../../Main/Accordion/Accordion";
import TextField from "../../../Main/TextField/TextField";
import { Timezone } from "../../../../types";
import "./style.scss";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";
import WarningTimezone from "./WarningTimezone";
import { useTimeState } from "../../../../state/time/TimeStateContext";

interface PinnedTimezone extends Timezone {
  title: string;
  isInvalid?: boolean;
}

type Props = {
  onChange: (tz: Timezone) => void;
}

const browserTimezone = getBrowserTimezone();

const TimezonesList: FC<Props> = ({ onChange }) => {
  const { isMobile } = useDeviceDetect();
  const { defaultTimezone } = useTimeState();

  const timezones = useMemo(() => getTimezoneList(), []);

  const [search, setSearch] = useState("");

  const pinnedTimezones = useMemo(() => [
    {
      title: `Default time (${defaultTimezone})`,
      region: defaultTimezone,
      utc: defaultTimezone ? getUTCByTimezone(defaultTimezone) : "UTC"
    },
    {
      title: browserTimezone.title,
      region: browserTimezone.region,
      utc: getUTCByTimezone(browserTimezone.region),
      isInvalid: !browserTimezone.isValid
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

  const handleChangeSearch = (val: string) => {
    setSearch(val);
  };

  const handleSetTimezone = (tz: Timezone) => {
    onChange(tz);
    setSearch("");
  };

  const createHandlerSetTimezone = (val: Timezone) => () => {
    handleSetTimezone(val);
  };

  return (
    <div
      className={classNames({
        "vm-list": true,
        "vm-timezones-list": true,
        "vm-timezones-list_mobile": isMobile,
      })}
    >
      <div className="vm-timezones-list-header">
        <div className="vm-timezones-list-header__search">
          <TextField
            label="Search"
            value={search}
            onChange={handleChangeSearch}
          />
        </div>
      </div>
      {pinnedTimezones.map((t, i) => t && (
        <div
          key={`${i}_${t.region}`}
          className="vm-list-item vm-timezones-item vm-timezones-list-group-options__item"
          onClick={createHandlerSetTimezone(t)}
        >
          <div className="vm-timezones-item__title">{t.title}{t.isInvalid && <WarningTimezone/>}</div>
          <div className="vm-timezones-item__utc">{t.utc}</div>
        </div>
      ))}
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
                  className="vm-list-item vm-timezones-item vm-timezones-list-group-options__item"
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
  );
};

export default TimezonesList;
