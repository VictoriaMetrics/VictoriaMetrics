import { FC, memo, useCallback } from "react";
import { createPortal } from "react-dom";
import DownloadLogsButton from "../../../DownloadLogsButton/DownloadLogsButton";
import JsonViewComponent from "../../../../../components/Views/JsonView/JsonView";
import { ViewProps } from "../../types";
import EmptyLogs from "../components/EmptyLogs/EmptyLogs";

const MemoizedJsonView = memo(JsonViewComponent);

const JsonView: FC<ViewProps> = ({ data, settingsRef }) => {
  const getLogs = useCallback(() => data, [data]);

  const renderSettings = () => {
    if (!settingsRef.current) return null;

    return createPortal(
      data.length > 0 && <DownloadLogsButton getLogs={getLogs} />,
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
