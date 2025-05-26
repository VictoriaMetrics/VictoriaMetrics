import { FC } from "preact/compat";
import "./style.scss";

const EmptyLogs: FC = () => {
  return (
    <div className="vm-explore-logs-body__empty">No logs found</div>
  );
};

export default EmptyLogs;
