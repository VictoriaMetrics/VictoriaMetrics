import { FC, useMemo } from "preact/compat";
import { Logs } from "../../../api/types";
import "./style.scss";
import classNames from "classnames";
import GroupLogsFieldRow from "./GroupLogsFieldRow";
import { useLocalStorageBoolean } from "../../../hooks/useLocalStorageBoolean";

interface Props {
  log: Logs;
  hideGroupButton?: boolean;
}

const GroupLogsFields: FC<Props> = ({ log, hideGroupButton }) => {
  const sortedFields = useMemo(() => {
    return Object.entries(log)
      .sort(([aKey], [bKey]) => aKey.localeCompare(bKey));
  }, [log]);

  const [disabledHovers] = useLocalStorageBoolean("LOGS_DISABLED_HOVERS");

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
              hideGroupButton={hideGroupButton}
            />
          ))}
        </tbody>
      </table>
    </div>
  );
};

export default GroupLogsFields;
