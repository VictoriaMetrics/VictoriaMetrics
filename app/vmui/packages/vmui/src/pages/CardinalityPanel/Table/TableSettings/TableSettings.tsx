import React, { FC, useEffect, useState, useRef, useMemo } from "preact/compat";
import { useSortedCategories } from "../../../../hooks/useSortedCategories";
import { InstantMetricResult } from "../../../../api/types";
import Button from "../../../../components/Main/Button/Button";
import { CloseIcon, SettingsIcon } from "../../../../components/Main/Icons";
import Popper from "../../../../components/Main/Popper/Popper";
import "./style.scss";
import Checkbox from "../../../../components/Main/Checkbox/Checkbox";
import Tooltip from "../../../../components/Main/Tooltip/Tooltip";

const title = "Display columns";

interface TableSettingsProps {
  data: InstantMetricResult[];
  defaultColumns?: string[]
  onChange: (arr: string[]) => void
}

const TableSettings: FC<TableSettingsProps> = ({ data, defaultColumns, onChange }) => {
  const buttonRef = useRef<HTMLDivElement>(null);
  const [openSettings, setOpenSettings] = useState(false);
  const columns = useSortedCategories(data);
  const disabledButton = useMemo(() => !columns.length, [columns]);
  const [checkedColumns, setCheckedColumns] = useState(columns.map(col => col.key));

  const handleChange = (key: string) => {
    setCheckedColumns(prev => checkedColumns.includes(key) ? prev.filter(col => col !== key) : [...prev, key]);
  };

  const handleClose = () => {
    setOpenSettings(false);
    setCheckedColumns(defaultColumns || columns.map(col => col.key));
  };

  const handleReset = () => {
    setOpenSettings(false);
    const value = columns.map(col => col.key);
    setCheckedColumns(value);
    onChange(value);
  };

  const handleApply = () => {
    setOpenSettings(false);
    onChange(checkedColumns);
  };

  const createHandlerChange = (key: string) => () => {
    handleChange(key);
  };

  const toggleOpenSettings = () => {
    setOpenSettings(prev => !prev);
  };

  useEffect(() => {
    setCheckedColumns(columns.map(col => col.key));
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
          />
        </div>
      </Tooltip>
      <Popper
        open={openSettings}
        onClose={handleClose}
        placement="bottom-right"
        buttonRef={buttonRef}
      >
        <div className="vm-table-settings-popper">
          <div className="vm-popper-header">
            <h3 className="vm-popper-header__title">
              {title}
            </h3>
            <Button
              onClick={handleClose}
              startIcon={<CloseIcon/>}
              size="small"
            />
          </div>
          <div className="vm-table-settings-popper-list">
            {columns.map(col => (
              <div
                className="vm-table-settings-popper-list__item"
                key={col.key}
              >
                <Checkbox
                  checked={checkedColumns.includes(col.key)}
                  onChange={createHandlerChange(col.key)}
                  label={col.key}
                />
              </div>
            ))}
          </div>
          <div className="vm-table-settings-popper__footer">
            <Button
              color="error"
              variant="outlined"
              size="small"
              onClick={handleReset}
            >
                Reset
            </Button>
            <Button
              variant="contained"
              size="small"
              onClick={handleApply}
            >
                apply
            </Button>
          </div>
        </div>
      </Popper>
    </div>
  );
};

export default TableSettings;
