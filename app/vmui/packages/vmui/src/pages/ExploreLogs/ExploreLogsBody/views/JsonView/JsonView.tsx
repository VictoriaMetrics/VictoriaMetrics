import React, { FC } from "preact/compat";
import DownloadLogsButton from "../../../DownloadLogsButton/DownloadLogsButton";
import { createPortal } from "preact/compat";
import JsonViewComponent from "../../../../../components/Views/JsonView/JsonView";
import { ViewProps } from "../../types";
import EmptyLogs from "../components/EmptyLogs/EmptyLogs";

const MemoizedJsonView = React.memo(JsonViewComponent);

const JsonView: FC<ViewProps> = ({ data, settingsRef }) => {
  const renderSettings = () => {
    if (!settingsRef.current) return null;

    return createPortal(
      data.length > 0 && <DownloadLogsButton logs={data} />,
      settingsRef.current
    );
  };

  if (!data.length) return <EmptyLogs />;

  return (
    <>
      {renderSettings()}
      <MemoizedJsonView data={data} />
    </>
  );
};

export default JsonView; 