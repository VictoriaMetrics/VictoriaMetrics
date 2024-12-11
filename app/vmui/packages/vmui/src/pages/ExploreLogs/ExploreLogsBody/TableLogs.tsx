import React, { FC, useMemo, useRef, useState } from "preact/compat";
import "./style.scss";
import Table from "../../../components/Table/Table";
import { Logs } from "../../../api/types";
import Pagination from "../../../components/Main/Pagination/Pagination";
import { useEffect } from "react";

interface TableLogsProps {
  logs: Logs[];
  displayColumns: string[];
  tableCompact: boolean;
  columns: string[];
  rowsPerPage: number;
}

const getColumnClass = (key: string) => {
  switch (key) {
    case "_time":
      return "vm-table-cell_logs-time";
    default:
      return "vm-table-cell_logs";
  }
};

const compactColumns = [{
  key: "_vmui_data",
  title: "Data",
  className: "vm-table-cell_logs vm-table-cell_pre"
}];

const TableLogs: FC<TableLogsProps> = ({ logs, displayColumns, tableCompact, columns, rowsPerPage }) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const [page, setPage] = useState(1);

  const rows = useMemo(() => {
    return logs.map((log) => {
      const _vmui_data = JSON.stringify(log, null, 2);
      return { ...log, _vmui_data };
    }) as Logs[];
  }, [logs]);

  const tableColumns = useMemo(() => {
    return columns.map((key) => ({
      key: key as keyof Logs,
      title: key,
      className: getColumnClass(key),
    }));
  }, [columns]);


  const filteredColumns = useMemo(() => {
    if (tableCompact) return compactColumns;
    if (!displayColumns?.length) return [];
    return tableColumns.filter(c => displayColumns.includes(c.key as string));
  }, [tableColumns, displayColumns, tableCompact]);

  const paginationOffset = useMemo(() => {
    const startIndex = (page - 1) * rowsPerPage;
    const endIndex = startIndex + rowsPerPage;
    return { startIndex, endIndex };
  }, [page, rowsPerPage]);

  const handlePageChange = (newPage: number) => {
    setPage(newPage);
    if (containerRef.current) {
      const y = containerRef.current.getBoundingClientRect().top + window.scrollY - 50;
      window.scrollTo({ top: y });
    }
  };

  useEffect(() => {
    setPage(1);
  }, [logs, rowsPerPage]);

  return (
    <>
      <div ref={containerRef}>
        <Table
          rows={rows}
          columns={filteredColumns}
          defaultOrderBy={"_time"}
          defaultOrderDir={"desc"}
          copyToClipboard={"_vmui_data"}
          paginationOffset={paginationOffset}
        />
      </div>
      <Pagination
        currentPage={page}
        totalItems={rows.length}
        itemsPerPage={rowsPerPage}
        onPageChange={handlePageChange}
      />
    </>
  );
};

export default TableLogs;
