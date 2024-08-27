import React, { FC, useEffect, useRef, useMemo } from "preact/compat";
import Button from "../../Main/Button/Button";
import { SearchIcon, SettingsIcon } from "../../Main/Icons";
import Popper from "../../Main/Popper/Popper";
import "./style.scss";
import Checkbox from "../../Main/Checkbox/Checkbox";
import Tooltip from "../../Main/Tooltip/Tooltip";
import Switch from "../../Main/Switch/Switch";
import { arrayEquals } from "../../../utils/array";
import classNames from "classnames";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import useBoolean from "../../../hooks/useBoolean";
import TextField from "../../Main/TextField/TextField";
import { KeyboardEvent, useState } from "react";

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

  const {
    value: showSearch,
    toggle: toggleShowSearch,
  } = useBoolean(false);

  const [searchColumn, setSearchColumn] = useState("");
  const [indexFocusItem, setIndexFocusItem] = useState(-1);

  const filteredColumns = useMemo(() => {
    if (!searchColumn) return columns;
    return columns.filter(col => col.includes(searchColumn));
  }, [columns, searchColumn]);

  const isAllChecked = useMemo(() => {
    return filteredColumns.every(col => defaultColumns.includes(col));
  }, [defaultColumns, filteredColumns]);

  const disabledButton = useMemo(() => !columns.length, [columns]);

  const handleChange = (key: string) => {
    onChangeColumns(defaultColumns.includes(key) ? defaultColumns.filter(col => col !== key) : [...defaultColumns, key]);
  };

  const toggleAllColumns = () => {
    if (isAllChecked) {
      onChangeColumns(defaultColumns.filter(col => !filteredColumns.includes(col)));
    } else {
      onChangeColumns(filteredColumns);
    }
  };

  const createHandlerChange = (key: string) => () => {
    handleChange(key);
  };

  const handleBlurSearch = () => {
    setIndexFocusItem(-1);
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    const arrowUp = e.key === "ArrowUp";
    const arrowDown = e.key === "ArrowDown";
    const enter = e.key === "Enter";
    if (arrowDown || arrowUp || enter) e.preventDefault();
    if (arrowDown) {
      setIndexFocusItem(prev => prev + 1 > filteredColumns.length - 1 ? prev : prev + 1);
    } else if (arrowUp) {
      setIndexFocusItem(prev => prev - 1 < 0 ? prev : prev - 1);
    } else if (enter) {
      handleChange(filteredColumns[indexFocusItem]);
    }
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
            ariaLabel={title}
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
            <div>
              <div className="vm-table-settings-popper-list-header">
                <h3 className="vm-table-settings-popper-list-header__title">Display columns</h3>
                <Tooltip title="search column">
                  <Button
                    color="primary"
                    variant="text"
                    onClick={toggleShowSearch}
                    startIcon={<SearchIcon/>}
                    ariaLabel="reset columns"
                  />
                </Tooltip>
              </div>
              {showSearch && (
                <TextField
                  placeholder={"search column"}
                  startIcon={<SearchIcon/>}
                  value={searchColumn}
                  onChange={setSearchColumn}
                  onBlur={handleBlurSearch}
                  onKeyDown={handleKeyDown}
                  type="search"
                />
              )}
              {!filteredColumns.length && (
                <p className="vm-table-settings-popper-list__no-found">No columns found</p>
              )}
              <div className="vm-table-settings-popper-list-header">
                {!!filteredColumns.length && (
                  <div className="vm-table-settings-popper-list__item vm-table-settings-popper-list__item_check_all">
                    <Checkbox
                      checked={isAllChecked}
                      onChange={toggleAllColumns}
                      label={isAllChecked ? "Uncheck all" : "Check all"}
                      disabled={tableCompact}
                    />
                  </div>
                )}
              </div>
              <div className="vm-table-settings-popper-list-columns">
                {filteredColumns.map((col, i) => (
                  <div
                    className={classNames({
                      "vm-table-settings-popper-list__item": true,
                      "vm-table-settings-popper-list__item_focus": i === indexFocusItem,
                    })}
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
          </div>
        </div>
      </Popper>
    </div>
  );
};

export default TableSettings;
