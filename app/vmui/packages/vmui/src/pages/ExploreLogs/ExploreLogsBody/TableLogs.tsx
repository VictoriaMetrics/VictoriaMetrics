import React, { FC, useMemo } from "preact/compat";
import "./style.scss";
import Table from "../../../components/Table/Table";
import { Logs } from "../../../api/types";

interface TableLogsProps {
  logs: Logs[];
  displayColumns: string[];
  tableCompact: boolean;
  columns: string[];
}

const TableLogs: FC<TableLogsProps> = ({ logs, displayColumns, tableCompact, columns }) => {
  const getColumnClass = (key: string) => {
    switch (key) {
      case "_time":
        return "vm-table-cell_logs-time";
      case "_vmui_data":
        return "vm-table-cell_logs vm-table-cell_pre";
      default:
        return "vm-table-cell_logs";
    }
  };

  const tableColumns = useMemo(() => {
    if (tableCompact) {
      return [{
        key: "_vmui_data",
        title: "Data",
        className: getColumnClass("_vmui_data")
      }];
    }
    return columns.map((key) => ({
      key: key as keyof Logs,
      title: key,
      className: getColumnClass(key),
    }));
  }, [tableCompact, columns]);


  const filteredColumns = useMemo(() => {
    if (!displayColumns?.length || tableCompact) return tableColumns;
    return tableColumns.filter(c => displayColumns.includes(c.key as string));
  }, [tableColumns, displayColumns, tableCompact]);

  return (
    <>
      <Table
        rows={logs}
        columns={filteredColumns}
        defaultOrderBy={"_vmui_time"}
        copyToClipboard={"_vmui_data"}
        paginationOffset={{ startIndex: 0, endIndex: Infinity }}
      />
    </>
  );
};

export default TableLogs;
