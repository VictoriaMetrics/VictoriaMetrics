import React, { FC, useState } from "react";
import JsonView from "../../../components/Views/JsonView/JsonView";
import { CodeIcon, TableIcon } from "../../../components/Main/Icons";
import Tabs from "../../../components/Main/Tabs/Tabs";
import "./style.scss";
import classNames from "classnames";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import Table from "../../../components/Table/Table";
import { Logs } from "../../../api/types";
import { useMemo } from "preact/compat";
import dayjs from "dayjs";
import TableSettings from "../../CardinalityPanel/Table/TableSettings/TableSettings";
import useBoolean from "../../../hooks/useBoolean";

export interface ExploreLogBodyProps {
  data: Logs[]
}

const tabs = ["table", "JSON"].map((t, i) => ({
  value: String(i),
  label: t,
  icon: i === 0 ? <TableIcon /> : <CodeIcon />
}));

const ExploreLogBody: FC<ExploreLogBodyProps> = ({ data }) => {
  const { isMobile } = useDeviceDetect();
  const [activeTab, setActiveTab] = useState(0);
  const [displayColumns, setDisplayColumns] = useState<string[]>([]);
  const { value: tableCompact, toggle: toggleTableCompact } = useBoolean(false);

  const getColumnClass = (key: string) => {
    switch (key) {
      case "time":
        return "vm-table-cell_no-wrap";
      case "msg":
        // TODO await answer
        // return "vm-table-cell_pre";
    }
  };

  const getMessage = (item: Logs) => {
    try {
      return JSON.stringify(JSON.parse(item._msg), null, 2);
    } catch (e) {
      return item._msg;
    }
  };

  const tableData = useMemo(() => {
    return data.map((item) => ({
      time: dayjs(item._time).tz().format("MMM DD, YYYY @ HH:mm:ss.SSS"),
      msg: getMessage(item),
      raw: JSON.stringify(item, null, 2),
      ...item,
    })) as Logs[];
  }, [data]);

  const columns = useMemo(() => {
    if (!tableData?.length) return [];
    if (tableCompact) return [{ key: "raw" as keyof Logs, title: "" }];
    const excludeColumns = ["raw", "_time", "_msg"];
    const keys = Object.keys(tableData[0]).map((key) => ({
      key: key as keyof Logs,
      title: key,
      className: getColumnClass(key),
    }));
    return keys.filter((c) => !excludeColumns.includes(c.key as string));
  }, [tableData, tableCompact]);

  const columnsKeys = useMemo(() => columns.map(c => c.key as string), [columns]);

  const filteredColumns = useMemo(() => {
    if (!displayColumns?.length) return columns;
    return columns.filter(c => displayColumns.includes(c.key as string));
  }, [columns, displayColumns]);

  const handleChangeTab = (val: string) => {
    setActiveTab(+val);
  };

  return (
    <div
      className={classNames({
        "vm-explore-log-body": true,
        "vm-block":  true,
        "vm-block_mobile": isMobile,
      })}
    >
      <div
        className={classNames({
          "vm-explore-log-body-header": true,
          "vm-section-header": true,
          "vm-explore-log-body-header_mobile": isMobile,
        })}
      >
        <div className="vm-section-header__tabs">
          <Tabs
            activeItem={String(activeTab)}
            items={tabs}
            onChange={handleChangeTab}
          />
        </div>
        {activeTab === 0 && (
          <TableSettings
            columns={columnsKeys}
            defaultColumns={displayColumns}
            onChangeColumns={setDisplayColumns}
            tableCompact={tableCompact}
            toggleTableCompact={toggleTableCompact}
          />
        )}
      </div>

      <div
        className={classNames({
          "vm-explore-log-body__table": true,
          "vm-explore-log-body__table_mobile": isMobile,
        })}
      >
        {!!data.length && (
          <>
            {activeTab === 0 && (
              <>
                <Table
                  rows={tableData}
                  columns={filteredColumns}
                  defaultOrderBy={"_time"}
                  copyToClipboard={"raw"}
                />
              </>
            )}
            {activeTab === 1 && (
              <JsonView data={data} />
            )}
          </>
        )}
      </div>
    </div>
  );
};

export default ExploreLogBody;
