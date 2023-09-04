import React, { FC, useMemo } from "preact/compat";
import "./style.scss";
import { Logs } from "../../../api/types";
import Accordion from "../../../components/Main/Accordion/Accordion";
import { groupByMultipleKeys } from "../../../utils/array";

interface TableLogsProps {
  logs: Logs[];
  columns: string[];
}

const GroupLogs: FC<TableLogsProps> = ({ logs, columns }) => {

  const groupData = useMemo(() => {
    const excludeColumns = ["_msg", "time", "data", "_time"];
    const keys = columns.filter((c) => !excludeColumns.includes(c as string));
    return groupByMultipleKeys(logs, keys);
  }, [logs]);

  return (
    <div className="vm-explore-logs-body-content">
      {groupData.map((item) => (
        <div
          className="vm-explore-logs-body-content-group"
          key={item.keys.join("")}
        >
          <Accordion
            defaultExpanded={true}
            title={(
              <div className="vm-explore-logs-body-content-group-keys">
                <span className="vm-explore-logs-body-content-group-keys__title">Group by:</span>
                {item.keys.map((key) => (
                  <div
                    className="vm-explore-logs-body-content-group-keys__key"
                    key={key}
                  >
                    {key}
                  </div>
                ))}
              </div>
            )}
          >
            <div className="vm-explore-logs-body-content-group-rows">
              {item.values.map((value) => (
                <div
                  className="vm-explore-logs-body-content-group-rows-item"
                  key={`${value._msg}${value._time}`}
                >
                  <div className="vm-explore-logs-body-content-group-rows-item__time">
                    {value.time}
                  </div>
                  <div className="vm-explore-logs-body-content-group-rows-item__msg">
                    {value._msg}
                  </div>
                </div>
              ))}
            </div>
          </Accordion>
        </div>
      ))}
    </div>
  );
};

export default GroupLogs;
