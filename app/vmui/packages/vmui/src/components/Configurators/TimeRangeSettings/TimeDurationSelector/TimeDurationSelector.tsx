import React, { FC } from "preact/compat";
import { relativeTimeOptions } from "../../../../utils/time";
import "./style.scss";
import classNames from "classnames";

interface TimeDurationSelector {
  setDuration: ({ duration, until, id }: {duration: string, until: Date, id: string}) => void;
  relativeTime: string;
}

const TimeDurationSelector: FC<TimeDurationSelector> = ({ relativeTime, setDuration }) => {
  console.log(relativeTimeOptions, relativeTime);
  return <div className="vm-time-duration">
    {relativeTimeOptions.map(({ id, duration, until, title }) =>
      <div
        className={classNames({
          "vm-time-duration__item": true,
          "vm-time-duration__item_active": id === relativeTime
        })}
        key={id}
        onClick={() => setDuration({ duration, until: until(), id })}
      >
        {title || duration}
      </div>)}
  </div>;
};

export default TimeDurationSelector;
