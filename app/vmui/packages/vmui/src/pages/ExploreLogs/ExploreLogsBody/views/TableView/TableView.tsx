import { FC, memo, useMemo, useState, useCallback } from "react";
import { createPortal } from "react-dom";
import DownloadLogsButton from "../../../DownloadLogsButton/DownloadLogsButton";
import "./style.scss";
import { ViewProps } from "../../types";
import useBoolean from "../../../../../hooks/useBoolean";
import useStateSearchParams from "../../../../../hooks/useStateSearchParams";
import TableLogs from "../../TableLogs";
import SelectLimit from "../../../../../components/Main/Pagination/SelectLimit/SelectLimit";
import TableSettings from "../../../../../components/Table/TableSettings/TableSettings";
import useSearchParamsFromObject from "../../../../../hooks/useSearchParamsFromObject";
import EmptyLogs from "../components/EmptyLogs/EmptyLogs";

const MemoizedTableView = memo(TableLogs);

const TableView: FC<ViewProps> = ({ data, settingsRef }) => {
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();
  const [displayColumns, setDisplayColumns] = useState<string[]>([]);
  const [rowsPerPage, setRowsPerPage] = useStateSearchParams(100, "rows_per_page");
  const { value: tableCompact, toggle: toggleTableCompact } = useBoolean(false);

  const columns = useMemo(() => {
    const keys = new Set<string>();
    for (const item of data) {
      for (const key in item) {
        keys.add(key);
      }
    }
    return Array.from(keys).sort((a,b) => a.localeCompare(b));
  }, [data]);

  const handleSetRowsPerPage = (limit: number) => {
    setRowsPerPage(limit);
    setSearchParamsFromKeys({ rows_per_page: limit });
  };

  const getLogs = useCallback(() => data, [data]);

  const renderSettings = () => {
    if (!settingsRef.current) return null;

    return createPortal(
      <div className="vm-table-view__settings">
        <SelectLimit
          limit={rowsPerPage}
          onChange={handleSetRowsPerPage}
        />
        <div className="vm-table-view__settings-buttons">
          {data.length > 0 && <DownloadLogsButton getLogs={getLogs} />}
          <TableSettings
            columns={columns}
            selectedColumns={displayColumns}
            onChangeColumns={setDisplayColumns}
            tableCompact={tableCompact}
            toggleTableCompact={toggleTableCompact}
          />
        </div>
      </div>,
      settingsRef.current
    );
  };

  if (!data.length) return <EmptyLogs />;

  return (
    <>
      {renderSettings()}
      <MemoizedTableView
        logs={data}
        displayColumns={displayColumns}
        tableCompact={tableCompact}
        columns={columns}
        rowsPerPage={Number(rowsPerPage)}
      />
    </>
  );
};

export default TableView;
