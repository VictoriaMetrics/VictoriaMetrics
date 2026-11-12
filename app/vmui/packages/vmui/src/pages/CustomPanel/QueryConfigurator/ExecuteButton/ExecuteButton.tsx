import { FC, ReactNode } from "preact/compat";
import Button from "../../../../components/Main/Button/Button";
import { PlayIcon, SpinnerIcon } from "../../../../components/Main/Icons";
import AutoRefreshControl from "../AutoRefreshControl/AutoRefreshControl";
import { useAutoRefreshInterval } from "../AutoRefreshControl/hooks/useAutoRefreshInterval";
import { REFRESH_DISABLE_OPTION } from "../AutoRefreshControl/constants";
import "./style.scss";

type Props = {
  label?: { run: string, cancel: string };
  icon?: ReactNode;
  isLoading?: boolean;
  onClick: () => void;
}

const ExecuteButton: FC<Props> = ({ label, icon, isLoading, onClick }) => {
  const { refreshInterval, setRefreshInterval } = useAutoRefreshInterval();
  const isAutoRefreshEnabled = refreshInterval !== REFRESH_DISABLE_OPTION;
  const refreshIntervalLabel = isAutoRefreshEnabled ? `(every ${refreshInterval})` : null;

  const isCancel = isAutoRefreshEnabled || isLoading;

  const handleClick = () => {
    if (isAutoRefreshEnabled) {
      setRefreshInterval(REFRESH_DISABLE_OPTION);
    }

    if (isLoading || !isAutoRefreshEnabled) {
      onClick();
    }
  };

  return (
    <div className="vm-execute-group">
      <Button
        className="vm-execute-group__execute-button"
        variant="contained"
        onClick={handleClick}
        startIcon={isLoading ? <SpinnerIcon/> : icon || <PlayIcon/>}
      >
        {isCancel ? label?.cancel || "Cancel" : label?.run || "Execute"}
        {isAutoRefreshEnabled && <span className="vm-execute-group__refresh-interval">{refreshIntervalLabel}</span>}
      </Button>
      <AutoRefreshControl/>
    </div>
  );
};

export default ExecuteButton;
