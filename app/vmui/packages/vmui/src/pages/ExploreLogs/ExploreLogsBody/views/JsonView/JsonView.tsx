import React, { FC, useMemo, useCallback, createPortal } from "preact/compat";
import DownloadLogsButton from "../../../DownloadLogsButton/DownloadLogsButton";
import JsonViewComponent from "../../../../../components/Views/JsonView/JsonView";
import { ViewProps } from "../../types";
import EmptyLogs from "../components/EmptyLogs/EmptyLogs";
import JsonViewSettings from "./JsonViewSettings/JsonViewSettings";
import { useSearchParams } from "react-router-dom";
import orderBy from "lodash.orderBy";
import "./style.scss";
import { Logs } from "../../../../../api/types";
import { SortDirection } from "./types";

const MemoizedJsonView = React.memo(JsonViewComponent);

const jsonQuerySortParam = "json_sort";
const fieldSortQueryParamName = "json_field_sort";

const JsonView: FC<ViewProps> = ({ data, settingsRef }) => {
  const getLogs = useCallback(() => data, [data]);

  const [searchParams] = useSearchParams();
  const sortParam = searchParams.get(jsonQuerySortParam);
  const fieldSortParam = searchParams.get(fieldSortQueryParamName) as SortDirection;

  const [sortField, sortDirection] = useMemo(() => {
    const [sortField, sortDirection] = sortParam?.split(":").map(decodeURIComponent) || [];
    return [sortField, sortDirection as "asc" | "desc" | undefined];
  }, [sortParam]);

  const fields = useMemo(() => {
    const keys = new Set(data.flatMap(Object.keys));
    return Array.from(keys);
  }, [data]);

  const orderedFieldsData = useMemo(() => {
    if (!fieldSortParam) return data;
    const orderedFields = fields.toSorted((a, b) => fieldSortParam === "asc" ? a.localeCompare(b): b.localeCompare(a));
    return data.map((item) => {
      return orderedFields.reduce((acc, field) => {
        if (item[field]) acc[field] = item[field];
        return acc;
      }, {} as Logs);
    });
  }, [fields, fieldSortParam, data]);

  const sortedData = useMemo(() => {
    if (!sortField || !sortDirection) return orderedFieldsData;
    return orderBy(orderedFieldsData, [sortField], [sortDirection]);
  }, [orderedFieldsData, sortField, sortDirection]);

  const renderSettings = () => {
    if (!settingsRef.current) return null;

    return createPortal(
      data.length > 0 && (
        <div className="vm-json-view__settings-container">
          <DownloadLogsButton getLogs={getLogs} />
          <JsonViewSettings
            fields={fields}
            sortQueryParamName={jsonQuerySortParam}
            fieldSortQueryParamName={fieldSortQueryParamName}
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
