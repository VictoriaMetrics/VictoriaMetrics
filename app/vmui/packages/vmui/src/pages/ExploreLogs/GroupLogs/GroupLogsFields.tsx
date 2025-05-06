import React, { FC, useMemo, useState } from "preact/compat";
import { Logs } from "../../../api/types";
import "./style.scss";
import classNames from "classnames";
import GroupLogsFieldRow from "./GroupLogsFieldRow";
import useEventListener from "../../../hooks/useEventListener";
import { getFromStorage } from "../../../utils/storage";

interface Props {
  log: Logs;
}

const GroupLogsFields: FC<Props> = ({ log }) => {
  const sortedFields = useMemo(() => {
    return Object.entries(log)
      .sort(([aKey], [bKey]) => aKey.localeCompare(bKey));
  }, [log]);

  const [disabledHovers, setDisabledHovers] = useState(!!getFromStorage("LOGS_DISABLED_HOVERS"));

  const handleUpdateStage = () => {
    const newValDisabledHovers = !!getFromStorage("LOGS_DISABLED_HOVERS");
    if (newValDisabledHovers !== disabledHovers) {
      setDisabledHovers(newValDisabledHovers);
    }
  };

  useEventListener("storage", handleUpdateStage);

  return (
    <div
      className={classNames({
        "vm-group-logs-row-fields": true,
        "vm-group-logs-row-fields_interactive": !disabledHovers
      })}
    >
      <table>
        <tbody>
          {sortedFields.map(([key, value]) => (
            <GroupLogsFieldRow
              key={key}
              field={key}
              value={value}
            />
          ))}
        </tbody>
      </table>
    </div>
  );
};

export default GroupLogsFields;
