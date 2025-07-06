import { FC, RefObject, useCallback, useRef } from "preact/compat";
import { createPortal } from "preact/compat";
import DownloadLogsButton from "../../../DownloadLogsButton/DownloadLogsButton";
import Button from "../../../../../components/Main/Button/Button";
import SelectLimit from "../../../../../components/Main/Pagination/SelectLimit/SelectLimit";
import { DeleteIcon, PauseIcon, PlayCircleOutlineIcon, SettingsIcon } from "../../../../../components/Main/Icons";
import Tooltip from "../../../../../components/Main/Tooltip/Tooltip";
import Modal from "../../../../../components/Main/Modal/Modal";
import Switch from "../../../../../components/Main/Switch/Switch";
import useBoolean from "../../../../../hooks/useBoolean";
import { Logs } from "../../../../../api/types";

interface LiveTailingSettingsProps {
  settingsRef: RefObject<HTMLDivElement>;
  rowsPerPage: number;
  handleSetRowsPerPage: (limit: number) => void;
  logs: Logs[];
  isPaused: boolean;
  handleResumeLiveTailing: () => void;
  pauseLiveTailing: () => void;
  clearLogs: () => void;
  isRawJsonView: boolean;
  onRawJsonViewChange: (value: boolean) => void;
}

const LiveTailingSettings: FC<LiveTailingSettingsProps> = ({
  settingsRef,
  rowsPerPage,
  handleSetRowsPerPage,
  logs,
  isPaused,
  handleResumeLiveTailing,
  pauseLiveTailing,
  clearLogs,
  isRawJsonView,
  onRawJsonViewChange
}) => {
  const settingButtonRef = useRef<HTMLDivElement>(null);
  const { value: isSettingsOpen, setFalse: closeSettings, setTrue: openSettings } = useBoolean(false);

  const getLogs = useCallback(() => logs.map(({ _log_id, ...log }) => log), [logs]);

  if (!settingsRef.current) return null;

  return createPortal(
    <div className="vm-live-tailing-view__settings">
      <SelectLimit
        limit={rowsPerPage}
        onChange={handleSetRowsPerPage}
        onOpenSelect={pauseLiveTailing}
      />
      <div className="vm-live-tailing-view__settings-buttons">
        {logs.length > 0 && <DownloadLogsButton getLogs={getLogs}/>}
        {isPaused ? (
          <Tooltip
            title={"Resume live tailing"}
          >
            <Button
              variant="text"
              color="primary"
              onClick={handleResumeLiveTailing}
              startIcon={<PlayCircleOutlineIcon/>}
              ariaLabel={"Resume live tailing"}
            />
          </Tooltip>
        ) : (
          <Tooltip
            title={"Pause live tailing"}
          >
            <Button
              variant="text"
              color="primary"
              onClick={pauseLiveTailing}
              startIcon={<PauseIcon/>}
              ariaLabel={"Pause live tailing"}
            />
          </Tooltip>
        )}
        <Tooltip
          title={"Clear logs"}
        >
          <Button
            variant="text"
            color="secondary"
            onClick={clearLogs}
            startIcon={<DeleteIcon/>}
            ariaLabel={"Clear logs"}
          />
        </Tooltip>
        <Tooltip
          title={"Settings"}
        >
          <Button
            ref={settingButtonRef}
            variant="text"
            color="secondary"
            onClick={openSettings}
            startIcon={<SettingsIcon/>}
            ariaLabel={"Settings"}
          />
        </Tooltip>
        {isSettingsOpen && <Modal
          onClose={closeSettings}
          title={"Live tailing settings"}
        >
          <div className="vm-live-tailing-view__settings-modal">
            <div className={"vm-live-tailing-view__settings-modal-item"}>
              <Switch
                label={"Raw JSON View"}
                value={isRawJsonView}
                onChange={onRawJsonViewChange}
              />
              <span className="vm-group-logs-configurator-item__info">
                When this option is enabled, logs will be displayed in raw JSON format. This improves performance and uses less CPU and memory.
              </span>
            </div>
          </div>
        </Modal>}
      </div>
    </div>,
    settingsRef.current
  );
};

export default LiveTailingSettings;
