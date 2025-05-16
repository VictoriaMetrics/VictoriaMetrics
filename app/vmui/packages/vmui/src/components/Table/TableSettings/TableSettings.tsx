import React, { FC, useEffect, useRef, useMemo } from "preact/compat";
import Button from "../../Main/Button/Button";
import { SearchIcon, SettingsIcon } from "../../Main/Icons";
import "./style.scss";
import Checkbox from "../../Main/Checkbox/Checkbox";
import Tooltip from "../../Main/Tooltip/Tooltip";
import Switch from "../../Main/Switch/Switch";
import { arrayEquals } from "../../../utils/array";
import classNames from "classnames";
import useBoolean from "../../../hooks/useBoolean";
import TextField, { TextFieldKeyboardEvent } from "../../Main/TextField/TextField";
import { useState } from "react";
import Modal from "../../Main/Modal/Modal";
import { useSearchParams } from "react-router-dom";

const title = "Table settings";

interface TableSettingsProps {
  columns: string[];
  selectedColumns?: string[];
  tableCompact?: boolean;
  toggleTableCompact?: () => void;
  onChangeColumns: (arr: string[]) => void
}

const TableSettings: FC<TableSettingsProps> = ({
  columns,
  selectedColumns = [],
  tableCompact,
  onChangeColumns,
  toggleTableCompact
}) => {
  const [searchParams, setSearchParams] = useSearchParams();

  const buttonRef = useRef<HTMLDivElement>(null);

  const {
    value: openSettings,
    toggle: toggleOpenSettings,
    setFalse: handleClose,
  } = useBoolean(false);

  const [searchColumn, setSearchColumn] = useState("");
  const [indexFocusItem, setIndexFocusItem] = useState(-1);

  const customColumns = useMemo(() => {
    return selectedColumns.filter(col => !columns.includes(col));
  }, [columns, selectedColumns]);

  const filteredColumns = useMemo(() => {
    const allColumns = customColumns.concat(columns);
    if (!searchColumn) return allColumns;
    return allColumns.filter(col => col.includes(searchColumn));
  }, [columns, customColumns, searchColumn]);

  const isAllChecked = useMemo(() => {
    return filteredColumns.every(col => selectedColumns.includes(col));
  }, [selectedColumns, filteredColumns]);

  const handleChangeDisplayColumns = (displayColumns: string[]) => {
    onChangeColumns(displayColumns);

    const updatedParams = new URLSearchParams(searchParams.toString());
    const isAllCheck = displayColumns.length === columns.length;

    if (isAllCheck) {
      updatedParams.delete("columns");
    } else {
      updatedParams.set("columns", displayColumns.map(encodeURIComponent).join(","));
    }

    setSearchParams(updatedParams);
  };

  const handleChange = (key: string) => {
    const displayColumns = selectedColumns.includes(key)
      ? selectedColumns.filter(col => col !== key)
      : [...selectedColumns, key];

    handleChangeDisplayColumns(displayColumns);
  };

  const toggleAllColumns = () => {
    if (isAllChecked) {
      handleChangeDisplayColumns(selectedColumns.filter(col => !filteredColumns.includes(col)));
    } else {
      handleChangeDisplayColumns(filteredColumns);
    }
  };

  const createHandlerChange = (key: string) => () => {
    handleChange(key);
  };

  const handleBlurSearch = () => {
    setIndexFocusItem(-1);
  };

  const handleKeyDown = (e: TextFieldKeyboardEvent) => {
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
    if (arrayEquals(columns, selectedColumns) || searchParams.has("columns")) return;
    onChangeColumns(columns);
  }, [columns]);

  useEffect(() => {
    const hasColumns = searchParams.has("columns");
    if (!hasColumns) return;
    const columnsParam = searchParams.get("columns") || "";
    const columnsArray = columnsParam.split(",").map(decodeURIComponent).filter(Boolean);
    onChangeColumns(columnsArray);
  }, []);

  return (
    <div className="vm-table-settings">
      <Tooltip title={title}>
        <div ref={buttonRef}>
          <Button
            variant="text"
            startIcon={<SettingsIcon/>}
            onClick={toggleOpenSettings}
            ariaLabel={title}
          />
        </div>
      </Tooltip>
      {openSettings && (
        <Modal
          title={title}
          className="vm-table-settings-modal"
          onClose={handleClose}
        >
          <div className="vm-table-settings-modal-section">
            <div className="vm-table-settings-modal-section__title">
              Customize columns
            </div>
            <div className="vm-table-settings-modal-columns">
              <div className="vm-table-settings-modal-columns__search">
                <TextField
                  placeholder={"Search columns"}
                  startIcon={<SearchIcon/>}
                  value={searchColumn}
                  onChange={setSearchColumn}
                  onBlur={handleBlurSearch}
                  onKeyDown={handleKeyDown}
                  type="search"
                />
              </div>
              <div className="vm-table-settings-modal-columns-list">
                {!!filteredColumns.length && (
                  <div className="vm-table-settings-modal-columns-list__item vm-table-settings-modal-columns-list__item_all">
                    <Checkbox
                      checked={isAllChecked}
                      onChange={toggleAllColumns}
                      label={isAllChecked ? "Uncheck all" : "Check all"}
                      disabled={tableCompact}
                    />
                  </div>
                )}
                {!filteredColumns.length && (
                  <div className="vm-table-settings-modal-columns-no-found">
                    <p className="vm-table-settings-modal-columns-no-found__info">
                      No columns found.
                    </p>
                  </div>
                )}
                {filteredColumns.map((col, i) => (
                  <div
                    className={classNames({
                      "vm-table-settings-modal-columns-list__item": true,
                      "vm-table-settings-modal-columns-list__item_focus": i === indexFocusItem,
                      "vm-table-settings-modal-columns-list__item_custom": customColumns.includes(col),
                    })}
                    key={col}
                  >
                    <Checkbox
                      checked={selectedColumns.includes(col)}
                      onChange={createHandlerChange(col)}
                      label={col}
                      disabled={tableCompact}
                    />
                  </div>
                ))}
              </div>
            </div>
          </div>
          {toggleTableCompact && tableCompact !== undefined && (
            <div className="vm-table-settings-modal-section">
              <div className="vm-table-settings-modal-section__title">
              Table view
              </div>
              <div className="vm-table-settings-modal-columns-list__item">
                <Switch
                  label={"Compact view"}
                  value={tableCompact}
                  onChange={toggleTableCompact}
                />
              </div>
            </div>
          )}
        </Modal>)}
    </div>
  );
};

export default TableSettings;
