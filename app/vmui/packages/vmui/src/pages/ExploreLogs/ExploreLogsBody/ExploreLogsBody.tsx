import React, { FC, useState, useMemo } from "preact/compat";
import JsonView from "../../../components/Views/JsonView/JsonView";
import { CodeIcon, ListIcon, TableIcon } from "../../../components/Main/Icons";
import Tabs from "../../../components/Main/Tabs/Tabs";
import "./style.scss";
import classNames from "classnames";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import { Logs } from "../../../api/types";
import dayjs from "dayjs";
import { useTimeState } from "../../../state/time/TimeStateContext";
import SelectLimit from "../../../components/Main/Pagination/SelectLimit/SelectLimit";
import useStateSearchParams from "../../../hooks/useStateSearchParams";
import useSearchParamsFromObject from "../../../hooks/useSearchParamsFromObject";
import { getFromStorage, saveToStorage } from "../../../utils/storage";
import TableSettings from "../../../components/Table/TableSettings/TableSettings";
import useBoolean from "../../../hooks/useBoolean";
import TableLogs from "./TableLogs";
import GroupLogs from "./GroupLogs";

export interface ExploreLogBodyProps {
  data: Logs[];
  loaded?: boolean;
}

enum DisplayType {
  group = "group",
  table = "table",
  json = "json",
}

const tabs = [
  { label: "Group", value: DisplayType.group, icon: <ListIcon /> },
  { label: "Table", value: DisplayType.table, icon: <TableIcon /> },
  { label: "JSON", value: DisplayType.json, icon: <CodeIcon /> },
];

const ExploreLogsBody: FC<ExploreLogBodyProps> = ({ data, loaded }) => {
  const { isMobile } = useDeviceDetect();
  const { timezone } = useTimeState();
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();
  const [limitRows, setLimitRows] = useStateSearchParams(getFromStorage("LOGS_LIMIT") || 50, "limit");

  const [activeTab, setActiveTab] = useStateSearchParams(DisplayType.group, "view");
  const [displayColumns, setDisplayColumns] = useState<string[]>([]);
  const { value: tableCompact, toggle: toggleTableCompact } = useBoolean(false);

  const logs = useMemo(() => data.map((item) => ({
    time: dayjs(item._time).tz().format("MMM DD, YYYY \nHH:mm:ss.SSS"),
    data: JSON.stringify(item, null, 2),
    ...item,
  })) as Logs[], [data, timezone]);

  const columns = useMemo(() => {
    if (!logs?.length) return [];
    const hideColumns = ["data", "_time"];
    const keys = new Set<string>();
    for (const item of logs) {
      for (const key in item) {
        keys.add(key);
      }
    }
    return Array.from(keys).filter((col) => !hideColumns.includes(col));
  }, [logs]);

  const handleChangeTab = (view: string) => {
    setActiveTab(view as DisplayType);
    setSearchParamsFromKeys({ view });
  };

  const handleChangeLimit = (limit: number) => {
    setLimitRows(limit);
    setSearchParamsFromKeys({ limit });
    saveToStorage("LOGS_LIMIT", `${limit}`);
  };

  return (
    <div
      className={classNames({
        "vm-explore-logs-body": true,
        "vm-block":  true,
        "vm-block_mobile": isMobile,
      })}
    >
      <div
        className={classNames({
          "vm-explore-logs-body-header": true,
          "vm-section-header": true,
          "vm-explore-logs-body-header_mobile": isMobile,
        })}
      >
        <div className="vm-section-header__tabs">
          <Tabs
            activeItem={String(activeTab)}
            items={tabs}
            onChange={handleChangeTab}
          />
        </div>
        {activeTab === DisplayType.table && (
          <div className="vm-explore-logs-body-header__settings">
            <SelectLimit
              limit={+limitRows}
              onChange={handleChangeLimit}
            />
            <TableSettings
              columns={columns}
              defaultColumns={displayColumns}
              onChangeColumns={setDisplayColumns}
              tableCompact={tableCompact}
              toggleTableCompact={toggleTableCompact}
            />
          </div>
        )}
      </div>

      <div
        className={classNames({
          "vm-explore-logs-body__table": true,
          "vm-explore-logs-body__table_mobile": isMobile,
        })}
      >
        {!data.length && (
          <div className="vm-explore-logs-body__empty">
            {loaded ? "No logs found" : "Run query to see logs"}
          </div>
        )}
        {!!data.length && (
          <>
            {activeTab === DisplayType.table && (
              <TableLogs
                logs={logs}
                limitRows={+limitRows}
                displayColumns={displayColumns}
                tableCompact={tableCompact}
                columns={columns}
              />
            )}
            {activeTab === DisplayType.group && (
              <GroupLogs
                logs={logs}
                columns={columns}
              />
            )}
            {activeTab === DisplayType.json && (
              <JsonView data={data} />
            )}
          </>
        )}
      </div>
    </div>
  );
};

export default ExploreLogsBody;
