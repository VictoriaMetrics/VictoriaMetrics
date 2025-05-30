import React, { FC, useMemo, useCallback, useState, createPortal } from "preact/compat";
import DownloadLogsButton from "../../../DownloadLogsButton/DownloadLogsButton";
import JsonViewComponent from "../../../../../components/Views/JsonView/JsonView";
import { ViewProps } from "../../types";
import EmptyLogs from "../components/EmptyLogs/EmptyLogs";
import JsonViewSettings from "./JsonViewSettings/JsonViewSettings";
import { useSearchParams } from "react-router-dom";
import orderBy from "lodash.orderBy";
import "./style.scss";

const MemoizedJsonView = React.memo(JsonViewComponent);
const jsonQuerySortParam = "json_sort";

const JsonView: FC<ViewProps> = ({ data, settingsRef }) => {
  const getLogs = useCallback(() => data, [data]);
  const [highlightedFields, setHighlightedFields] = useState<Record<string, string>>({});

  const [searchParams] = useSearchParams();
  const sortParam = searchParams.get(jsonQuerySortParam);

  const [sortField, sortDirection] = useMemo(() => {
    const [sortField, sortDirection] = sortParam ? sortParam.split(":").map(decodeURIComponent) : [undefined, undefined];
    return [sortField, sortDirection as "asc" | "desc" | undefined];
  }, [sortParam]);

  const fields = useMemo(() => {
    const keys = new Set<string>();
    for (const item of data) {
      for (const key in item) {
        keys.add(key);
      }
    }
    return Array.from(keys);
  }, [data]);

  const sortedData = useMemo(() => {
    if (!sortField || !sortDirection) return data;
    return orderBy(data, [sortField], [sortDirection]);
  }, [data, sortField, sortDirection]);

  const renderSettings = () => {
    if (!settingsRef.current) return null;

    return createPortal(
      data.length > 0 && (
        <div className="vm-json-view__settings-container">
          <DownloadLogsButton getLogs={getLogs} />
          <JsonViewSettings
            fields={fields}
            sortQueryParamName={jsonQuerySortParam}
            highlightedFields={highlightedFields}
            setHighlightedFields={setHighlightedFields}
            logsCount={data.length}
          />
        </div>
      ),
      settingsRef.current
    );
  };

  if (!data.length) return <EmptyLogs />;

  return (
    <>
      {renderSettings()}
      <MemoizedJsonView
        data={sortedData}
      />
    </>
  );
};

export default JsonView; 
