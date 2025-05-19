import React, { FC } from "preact/compat";
import GroupLogs from "../../../GroupLogs/GroupLogs";
import { ViewProps } from "../../types";
import EmptyLogs from "../components/EmptyLogs/EmptyLogs";

const MemoizedGroupLogs = React.memo(GroupLogs);

const GroupView: FC<ViewProps> = ({ data, settingsRef }) => {
  if (!data.length) return <EmptyLogs />;

  return (
    <>
      <MemoizedGroupLogs
        logs={data}
        settingsRef={settingsRef}
      />
    </>
  );
};

export default GroupView; 
