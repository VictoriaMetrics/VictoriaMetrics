import { FC } from "preact/compat";
import { relativeTimeOptions } from "../../../../utils/time";
import "./style.scss";
import classNames from "classnames";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";

interface TimeDurationSelector {
  setDuration: ({ duration, until, id }: {duration: string, until: Date, id: string}) => void;
  relativeTime: string;
}

const TimeDurationSelector: FC<TimeDurationSelector> = ({ relativeTime, setDuration }) => {
  const { isMobile } = useDeviceDetect();

  const createHandlerClick = (value: { duration: string, until: Date, id: string }) => () => {
    setDuration(value);
  };

  return (
    <div
      className={classNames({
        "vm-time-duration": true,
        "vm-time-duration_mobile": isMobile,
      })}
    >
      {relativeTimeOptions.map(({ id, duration, until, title }) => (
        <div
          className={classNames({
            "vm-list-item": true,
            "vm-list-item_mobile": isMobile,
            "vm-list-item_active": id === relativeTime
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
