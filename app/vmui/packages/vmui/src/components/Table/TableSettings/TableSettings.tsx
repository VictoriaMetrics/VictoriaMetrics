import React, { FC, useEffect, useRef, useMemo } from "preact/compat";
import Button from "../../Main/Button/Button";
import { RestartIcon, SettingsIcon } from "../../Main/Icons";
import Popper from "../../Main/Popper/Popper";
import "./style.scss";
import Checkbox from "../../Main/Checkbox/Checkbox";
import Tooltip from "../../Main/Tooltip/Tooltip";
import Switch from "../../Main/Switch/Switch";
import { arrayEquals } from "../../../utils/array";
import classNames from "classnames";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import useBoolean from "../../../hooks/useBoolean";

const title = "Table settings";

interface TableSettingsProps {
  columns: string[];
  defaultColumns?: string[];
  tableCompact: boolean;
  toggleTableCompact: () => void;
  onChangeColumns: (arr: string[]) => void
}

const TableSettings: FC<TableSettingsProps> = ({
  columns,
  defaultColumns = [],
  tableCompact,
  onChangeColumns,
  toggleTableCompact
}) => {
  const { isMobile } = useDeviceDetect();

  const buttonRef = useRef<HTMLDivElement>(null);

  const {
    value: openSettings,
    toggle: toggleOpenSettings,
    setFalse: handleClose,
  } = useBoolean(false);

  const disabledButton = useMemo(() => !columns.length, [columns]);

  const handleChange = (key: string) => {
    onChangeColumns(defaultColumns.includes(key) ? defaultColumns.filter(col => col !== key) : [...defaultColumns, key]);
  };

  const handleResetColumns = () => {
    handleClose();
    onChangeColumns(columns);
  };

  const createHandlerChange = (key: string) => () => {
    handleChange(key);
  };

  useEffect(() => {
    if (arrayEquals(columns, defaultColumns)) return;
    onChangeColumns(columns);
  }, [columns]);

  return (
    <div className="vm-table-settings">
      <Tooltip title={title}>
        <div ref={buttonRef}>
          <Button
            variant="text"
            startIcon={<SettingsIcon/>}
            onClick={toggleOpenSettings}
            disabled={disabledButton}
            ariaLabel="table settings"
          />
        </div>
      </Tooltip>
      <Popper
        open={openSettings}
        onClose={handleClose}
        placement="bottom-right"
        buttonRef={buttonRef}
        title={title}
      >
        <div
          className={classNames({
            "vm-table-settings-popper": true,
            "vm-table-settings-popper_mobile": isMobile
          })}
        >
          <div className="vm-table-settings-popper-list vm-table-settings-popper-list_first">
            <Switch
              label={"Compact view"}
              value={tableCompact}
              onChange={toggleTableCompact}
            />
          </div>
          <div className="vm-table-settings-popper-list">
            <div className="vm-table-settings-popper-list-header">
              <h3 className="vm-table-settings-popper-list-header__title">Display columns</h3>
              <Tooltip title="Reset to default">
                <Button
                  color="primary"
                  variant="text"
                  size="small"
                  onClick={handleResetColumns}
                  startIcon={<RestartIcon/>}
                  ariaLabel="reset columns"
                />
              </Tooltip>
            </div>
            {columns.map(col => (
              <div
                className="vm-table-settings-popper-list__item"
                key={col}
              >
                <Checkbox
                  checked={defaultColumns.includes(col)}
                  onChange={createHandlerChange(col)}
                  label={col}
                  disabled={tableCompact}
                />
              </div>
            ))}
          </div>
        </div>
      </Popper>
    </div>
  );
};

export default TableSettings;
