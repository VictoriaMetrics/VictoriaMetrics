import React, { FC } from "preact/compat";
import { relativeTimeOptions } from "../../../../utils/time";
import "./style.scss";
import classNames from "classnames";

interface TimeDurationSelector {
  setDuration: ({ duration, until, id }: {duration: string, until: Date, id: string}) => void;
  relativeTime: string;
}

const TimeDurationSelector: FC<TimeDurationSelector> = ({ relativeTime, setDuration }) => {

  const createHandlerClick = (value: { duration: string, until: Date, id: string }) => () => {
    setDuration(value);
  };

  return (
    <div className="vm-time-duration">
      {relativeTimeOptions.map(({ id, duration, until, title }) => (
        <div
          className={classNames({
            "vm-list__item": true,
            "vm-list__item_active": id === relativeTime
          })}
          key={id}
          onClick={createHandlerClick({ duration, until: until(), id })}
        >
          {title || duration}
        </div>
      ))}
    </div>
  );
};

export default TimeDurationSelector;
