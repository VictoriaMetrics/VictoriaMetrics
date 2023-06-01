import React, { FC, useRef, useState } from "preact/compat";
import { KeyboardEvent } from "react";
import { ErrorTypes } from "../../../types";
import TextField from "../../Main/TextField/TextField";
import Autocomplete from "../../Main/Autocomplete/Autocomplete";
import "./style.scss";
import { QueryStats } from "../../../api/types";
import Tooltip from "../../Main/Tooltip/Tooltip";
import { WarningIcon } from "../../Main/Icons";

export interface QueryEditorProps {
  onChange: (query: string) => void;
  onEnter: () => void;
  onArrowUp: () => void;
  onArrowDown: () => void;
  value: string;
  oneLiner?: boolean;
  autocomplete: boolean;
  error?: ErrorTypes | string;
  stats?: QueryStats;
  options: string[];
  label: string;
  disabled?: boolean
}

const QueryEditor: FC<QueryEditorProps> = ({
  value,
  onChange,
  onEnter,
  onArrowUp,
  onArrowDown,
  autocomplete,
  error,
  stats,
  options,
  label,
  disabled = false
}) => {

  const [openAutocomplete, setOpenAutocomplete] = useState(false);
  const autocompleteAnchorEl = useRef<HTMLDivElement>(null);
  const showSeriesFetchedWarning = stats?.seriesFetched === "0" && !stats.resultLength;

  const handleSelect = (val: string) => {
    onChange(val);
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    const { key, ctrlKey, metaKey, shiftKey } = e;

    const value = (e.target as HTMLTextAreaElement).value || "";
    const isMultiline = value.split("\n").length > 1;

    const ctrlMetaKey = ctrlKey || metaKey;
    const arrowUp = key === "ArrowUp";
    const arrowDown = key === "ArrowDown";
    const enter = key === "Enter";

    // prev value from history
    if (arrowUp && ctrlMetaKey) {
      e.preventDefault();
      onArrowUp();
    }

    // next value from history
    if (arrowDown && ctrlMetaKey) {
      e.preventDefault();
      onArrowDown();
    }

    if (enter && openAutocomplete) {
      e.preventDefault();
    }

    // execute query
    if (enter && !shiftKey && (!isMultiline || ctrlMetaKey) && !openAutocomplete) {
      e.preventDefault();
      onEnter();
    }
  };

  const handleChangeFoundOptions = (val: string[]) => {
    setOpenAutocomplete(!!val.length);
  };

  return <div
    className="vm-query-editor"
    ref={autocompleteAnchorEl}
  >
    <TextField
      value={value}
      label={label}
      type={"textarea"}
      autofocus={!!value}
      error={error}
      onKeyDown={handleKeyDown}
      onChange={onChange}
      disabled={disabled}
      inputmode={"search"}
    />
    {autocomplete && (
      <Autocomplete
        disabledFullScreen
        value={value}
        options={options}
        anchor={autocompleteAnchorEl}
        onSelect={handleSelect}
        onFoundOptions={handleChangeFoundOptions}
      />
    )}
    {showSeriesFetchedWarning && (
      <div className="vm-query-editor-warning">
        <Tooltip
          placement="bottom-right"
          title={(
            <span className="vm-query-editor-warning__tooltip">
              {`No match! 
              This query hasn't selected any time series from database.
              Either the requested metrics are missing in the database, 
              or there is a typo in series selector.`}
            </span>
          )}
        >
          <WarningIcon/>
        </Tooltip>
      </div>
    )}
  </div>;
};

export default QueryEditor;
