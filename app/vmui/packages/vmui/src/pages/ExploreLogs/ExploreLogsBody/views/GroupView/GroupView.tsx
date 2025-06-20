import { FC, memo } from "react";
import GroupLogs from "../../../GroupLogs/GroupLogs";
import { ViewProps } from "../../types";
import EmptyLogs from "../components/EmptyLogs/EmptyLogs";

const MemoizedGroupLogs = memo(GroupLogs);

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
