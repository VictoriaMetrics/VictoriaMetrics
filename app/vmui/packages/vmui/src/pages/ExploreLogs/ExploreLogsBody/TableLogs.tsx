import React, { FC, useMemo } from "preact/compat";
import "./style.scss";
import Table from "../../../components/Table/Table";
import { Logs } from "../../../api/types";
import useStateSearchParams from "../../../hooks/useStateSearchParams";
import useSearchParamsFromObject from "../../../hooks/useSearchParamsFromObject";
import PaginationControl from "../../../components/Main/Pagination/PaginationControl/PaginationControl";

interface TableLogsProps {
  logs: Logs[];
  limitRows: number;
  displayColumns: string[];
  tableCompact: boolean;
  columns: string[];
}

const TableLogs: FC<TableLogsProps> = ({ logs, limitRows, displayColumns, tableCompact, columns }) => {
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();
  const [page, setPage] = useStateSearchParams(1, "page");

  const getColumnClass = (key: string) => {
    switch (key) {
      case "time":
        return "vm-table-cell_logs-time";
      case "data":
        return "vm-table-cell_logs vm-table-cell_pre";
      default:
        return "vm-table-cell_logs";
    }
  };

  // TODO: Remove when pagination is implemented on the backend.
  const paginationOffset = useMemo(() => {
    const startIndex = (page - 1) * Number(limitRows);
    const endIndex = startIndex + Number(limitRows);
    return {
      startIndex,
      endIndex
    };
  }, [page, limitRows]);

  const tableColumns = useMemo(() => {
    if (tableCompact) {
      return [{
        key: "data",
        title: "Data",
        className: getColumnClass("data")
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

  const handleChangePage = (page: number) => {
    setPage(page);
    setSearchParamsFromKeys({ page });
  };

  return (
    <>
      <Table
        rows={logs}
        columns={filteredColumns}
        defaultOrderBy={"time"}
        copyToClipboard={"data"}
        paginationOffset={paginationOffset}
      />
      <PaginationControl
        page={page}
        limit={+limitRows}
        // TODO: Remove .slice() when pagination is implemented on the backend.
        length={logs.slice(paginationOffset.startIndex, paginationOffset.endIndex).length}
        onChange={handleChangePage}
      />
    </>
  );
};

export default TableLogs;
