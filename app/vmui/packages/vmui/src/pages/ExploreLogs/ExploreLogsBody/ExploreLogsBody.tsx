import React, { FC, useState, useMemo, useRef } from "preact/compat";
import { CodeIcon, CurlyBracketIcon, ListIcon, TableIcon } from "../../../components/Main/Icons";
import Tabs from "../../../components/Main/Tabs/Tabs";
import "./style.scss";
import classNames from "classnames";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import { Logs } from "../../../api/types";
import useStateSearchParams from "../../../hooks/useStateSearchParams";
import useSearchParamsFromObject from "../../../hooks/useSearchParamsFromObject";
import TableSettings from "../../../components/Table/TableSettings/TableSettings";
import useBoolean from "../../../hooks/useBoolean";
import TableLogs from "./TableLogs";
import GroupLogs from "../GroupLogs/GroupLogs";
import JsonView from "../../../components/Views/JsonView/JsonView";
import LineLoader from "../../../components/Main/LineLoader/LineLoader";
import SelectLimit from "../../../components/Main/Pagination/SelectLimit/SelectLimit";
import DownloadLogsButton from "../DownloadLogsButton/DownloadLogsButton";

const MemoizedTableLogs = React.memo(TableLogs);
const MemoizedGroupLogs = React.memo(GroupLogs);
const MemoizedJsonView = React.memo(JsonView);

export interface ExploreLogBodyProps {
  data: Logs[];
  isLoading: boolean;
}

enum DisplayType {
  group = "group",
  table = "table",
  compactJSON = "compactJSON",
  json = "json",
}

const tabs = [
  { label: "Group", value: DisplayType.group, icon: <ListIcon/> },
  { label: "Table", value: DisplayType.table, icon: <TableIcon/> },
  { label: "JSON", value: DisplayType.compactJSON, icon: <CodeIcon/> },
  { label: "Raw JSON", value: DisplayType.json, icon: <CurlyBracketIcon/> },
];

const ExploreLogsBody: FC<ExploreLogBodyProps> = ({ data, isLoading }) => {
  const { isMobile } = useDeviceDetect();
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();
  const groupSettingsRef = useRef<HTMLDivElement>(null);

  const [activeTab, setActiveTab] = useStateSearchParams(DisplayType.group, "view");
  const [displayColumns, setDisplayColumns] = useState<string[]>([]);
  const [rowsPerPage, setRowsPerPage] = useStateSearchParams(1000, "rows_per_page");
  const { value: tableCompact, toggle: toggleTableCompact } = useBoolean(false);

  const columns = useMemo(() => {
    if (!data?.length || (activeTab !== DisplayType.table && activeTab !== DisplayType.compactJSON)) return [];
    const keys = new Set<string>();
    for (const item of data) {
      for (const key in item) {
        keys.add(key);
      }
    }
    return Array.from(keys);
  }, [data, activeTab]);

  const handleChangeTab = (view: string) => {
    setActiveTab(view as DisplayType);
    setSearchParamsFromKeys({ view });
  };

  const handleSetRowsPerPage = (limit: number) => {
    setRowsPerPage(limit);
    setSearchParamsFromKeys({ rows_per_page: limit });
  };

  return (
    <div
      className={classNames({
        "vm-explore-logs-body": true,
        "vm-block": true,
        "vm-block_mobile": isMobile,
      })}
    >
      {isLoading && <LineLoader/>}
      <div
        className={classNames({
          "vm-explore-logs-body-header": true,
          "vm-section-header": true,
          "vm-explore-logs-body-header_mobile": isMobile,
        })}
      >
        <div
          className={classNames({
            "vm-section-header__tabs": true,
            "vm-explore-logs-body-header__tabs_mobile": isMobile,
          })}
        >
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
            <SelectLimit
              limit={rowsPerPage}
              onChange={handleSetRowsPerPage}
            />
            <div className="vm-explore-logs-body-header__table-settings">
              {data.length > 0 && <DownloadLogsButton logs={data}/>}
              <TableSettings
                columns={columns}
                selectedColumns={displayColumns}
                onChangeColumns={setDisplayColumns}
              />
            </div>
          </div>
        )}
        {activeTab === DisplayType.group && (
          <>
            <div
              className="vm-explore-logs-body-header__settings"
              ref={groupSettingsRef}
            />
          </>
        )}
        {activeTab === DisplayType.compactJSON && !!data.length && (
          <div className="vm-explore-logs-body-header__settings">
            <SelectLimit
              limit={rowsPerPage}
              onChange={handleSetRowsPerPage}
            />
            <DownloadLogsButton logs={data}/>
          </div>
        )}
        {activeTab === DisplayType.json && !!data.length && (
          <DownloadLogsButton logs={data}/>
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
              <MemoizedTableLogs
                logs={data}
                displayColumns={displayColumns}
                tableCompact={tableCompact}
                columns={columns}
                rowsPerPage={Number(rowsPerPage)}
              />
            )}
            {activeTab === DisplayType.compactJSON && (
              <MemoizedTableLogs
                logs={data}
                displayColumns={displayColumns}
                tableCompact={true}
                columns={columns}
                rowsPerPage={Number(rowsPerPage)}
              />
            )}
            {activeTab === DisplayType.group && (
              <MemoizedGroupLogs
                logs={data}
                settingsRef={groupSettingsRef}
              />
            )}
            {activeTab === DisplayType.json && (
              <MemoizedJsonView data={data}/>
            )}
          </>
        )}
      </div>
    </div>
  );
};

export default ExploreLogsBody;
