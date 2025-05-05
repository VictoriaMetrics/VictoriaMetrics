import React, { FC } from "preact/compat";
import DownloadLogsButton from "../../../DownloadLogsButton/DownloadLogsButton";
import { createPortal } from "preact/compat";
import GroupLogs from "../../../GroupLogs/GroupLogs";
import { ViewProps } from "../../types";
import EmptyLogs from "../components/EmptyLogs/EmptyLogs";

const MemoizedGroupLogs = React.memo(GroupLogs);

const GroupView: FC<ViewProps> = ({ data, settingsRef }) => {
  const renderSettings = () => {
    if (!settingsRef.current) return null;
    return createPortal(
      <div className="vm-group-view__settings">
        {data.length > 0 && <DownloadLogsButton logs={data} />}
      </div>,
      settingsRef.current
    );
  };

  if (!data.length) return <EmptyLogs />;

  return (
    <>
      {renderSettings()}
      <MemoizedGroupLogs
        logs={data}
        settingsRef={settingsRef}
      />
    </>
  );
};

export default GroupView; 