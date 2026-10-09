import { FC, useState } from "preact/compat";
import { getCustomRelativeTimeId, normalizeDuration, relativeTimeOptions } from "../../../../utils/time";
import "./style.scss";
import classNames from "classnames";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";
import TextField from "../../../Main/TextField/TextField";
import { getRecentDurations, saveRecentDuration } from "./utils";
import Button from "../../../Main/Button/Button";
import { DoneIcon } from "../../../Main/Icons";

interface TimeDurationSelector {
  setDuration: ({ duration, until, id }: {duration: string, until: Date, id: string}) => void;
  relativeTime: string;
}

const TimeDurationSelector: FC<TimeDurationSelector> = ({ relativeTime, setDuration }) => {
  const { isMobile } = useDeviceDetect();
  const [customDuration, setCustomDuration] = useState("");
  const [recentDurations, setRecentDurations] = useState(getRecentDurations);
  const [error, setError] = useState("");

  const createHandlerClick = (value: { duration: string, until: Date, id: string }) => () => {
    setDuration(value);
  };

  const handleCustomDurationChange = (value: string) => {
    setCustomDuration(value);
    setError("");
  };

  const handleCustomDurationSubmit = () => {
    const duration = normalizeDuration(customDuration);
    if (!duration) {
      setError("Enter a valid duration, for example 5m, 4h, or 10d");
      return;
    }

    setRecentDurations(saveRecentDuration(duration));
    setCustomDuration("");
    setError("");
    setDuration({
      duration,
      until: new Date(),
      id: getCustomRelativeTimeId(duration),
    });
  };

  return (
    <div
      className={classNames({
        "vm-time-duration": true,
        "vm-time-duration_mobile": isMobile,
      })}
    >
      <div className="vm-time-duration-custom">
        <TextField
          label="Last"
          placeholder="e.g. 5m, 4h, 10d"
          value={customDuration}
          error={error}
          onChange={handleCustomDurationChange}
          onEnter={handleCustomDurationSubmit}
          endIcon={(
            <Button
              size="small"
              variant="text"
              color="primary"
              startIcon={<DoneIcon/>}
              onClick={handleCustomDurationSubmit}
              ariaLabel="apply custom time range"
            />
          )}
        />
      </div>
      {recentDurations.map(duration => {
        const id = getCustomRelativeTimeId(duration);
        return (
          <div
            className={classNames({
              "vm-list-item": true,
              "vm-list-item_mobile": isMobile,
              "vm-list-item_active": id === relativeTime
            })}
            key={id}
            onClick={createHandlerClick({ duration, until: new Date(), id })}
          >
            Last {duration}
          </div>
        );
      })}
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
