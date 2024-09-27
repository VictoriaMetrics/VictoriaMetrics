import React, { FC, useState, useMemo, useRef } from "preact/compat";
import JsonView from "../../../components/Views/JsonView/JsonView";
import { CodeIcon, ListIcon, TableIcon } from "../../../components/Main/Icons";
import Tabs from "../../../components/Main/Tabs/Tabs";
import "./style.scss";
import classNames from "classnames";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import { Logs } from "../../../api/types";
import dayjs from "dayjs";
import { useTimeState } from "../../../state/time/TimeStateContext";
import useStateSearchParams from "../../../hooks/useStateSearchParams";
import useSearchParamsFromObject from "../../../hooks/useSearchParamsFromObject";
import TableSettings from "../../../components/Table/TableSettings/TableSettings";
import useBoolean from "../../../hooks/useBoolean";
import TableLogs from "./TableLogs";
import GroupLogs from "../GroupLogs/GroupLogs";
import { DATE_TIME_FORMAT } from "../../../constants/date";
import { marked } from "marked";

export interface ExploreLogBodyProps {
  data: Logs[];
}

enum DisplayType {
  group = "group",
  table = "table",
  json = "json",
}

const tabs = [
  { label: "Group", value: DisplayType.group, icon: <ListIcon/> },
  { label: "Table", value: DisplayType.table, icon: <TableIcon/> },
  { label: "JSON", value: DisplayType.json, icon: <CodeIcon/> },
];

const ExploreLogsBody: FC<ExploreLogBodyProps> = ({ data }) => {
  const { isMobile } = useDeviceDetect();
  const { timezone } = useTimeState();
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();
  const groupSettingsRef = useRef<HTMLDivElement>(null);

  const [activeTab, setActiveTab] = useStateSearchParams(DisplayType.group, "view");
  const [displayColumns, setDisplayColumns] = useState<string[]>([]);
  const { value: tableCompact, toggle: toggleTableCompact } = useBoolean(false);

  const logs = useMemo(() => data.map((item) => ({
    ...item,
    _vmui_time: item._time ? dayjs(item._time).tz().format(`${DATE_TIME_FORMAT}.SSS`) : "",
    _vmui_data: JSON.stringify(item, null, 2),
    _vmui_markdown: item._msg ? marked(item._msg.replace(/```/g, "\n```\n")) as string : ""
  })) as Logs[], [data, timezone]);

  const columns = useMemo(() => {
    if (!logs?.length) return [];
    const hideColumns = ["_vmui_data", "_vmui_time", "_vmui_markdown"];
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

  return (
    <div
      className={classNames({
        "vm-explore-logs-body": true,
        "vm-block": true,
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
          <div className="vm-explore-logs-body-header__log-info">
            Total logs returned: <b>{data.length}</b>
          </div>
        </div>
        {activeTab === DisplayType.table && (
          <div className="vm-explore-logs-body-header__settings">
            <TableSettings
              columns={columns}
              selectedColumns={displayColumns}
              onChangeColumns={setDisplayColumns}
              tableCompact={tableCompact}
              toggleTableCompact={toggleTableCompact}
            />
          </div>
        )}
        {activeTab === DisplayType.group && (
          <div
            className="vm-explore-logs-body-header__settings"
            ref={groupSettingsRef}
          />
        )}
      </div>

      <div
        className={classNames({
          "vm-explore-logs-body__table": true,
          "vm-explore-logs-body__table_mobile": isMobile,
        })}
      >
        {!data.length && <div className="vm-explore-logs-body__empty">No logs found</div>}
        {!!data.length && (
          <>
            {activeTab === DisplayType.table && (
              <TableLogs
                logs={logs}
                displayColumns={displayColumns}
                tableCompact={tableCompact}
                columns={columns}
              />
            )}
            {activeTab === DisplayType.group && (
              <GroupLogs
                logs={logs}
                columns={columns}
                settingsRef={groupSettingsRef}
              />
            )}
            {activeTab === DisplayType.json && (
              <JsonView data={data}/>
            )}
          </>
        )}
      </div>
    </div>
  );
};

export default ExploreLogsBody;
