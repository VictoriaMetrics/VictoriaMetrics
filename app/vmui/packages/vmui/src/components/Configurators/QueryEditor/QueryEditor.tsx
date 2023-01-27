import React, { FC, useRef, useState } from "preact/compat";
import { KeyboardEvent } from "react";
import { ErrorTypes } from "../../../types";
import TextField from "../../Main/TextField/TextField";
import Autocomplete from "../../Main/Autocomplete/Autocomplete";
import "./style.scss";

export interface QueryEditorProps {
  onChange: (query: string) => void;
  onEnter: () => void;
  onArrowUp: () => void;
  onArrowDown: () => void;
  value: string;
  oneLiner?: boolean;
  autocomplete: boolean;
  error?: ErrorTypes | string;
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
  options,
  label,
  disabled = false
}) => {

  const [openAutocomplete, setOpenAutocomplete] = useState(false);
  const autocompleteAnchorEl = useRef<HTMLDivElement>(null);

  const handleSelect = (val: string) => {
    onChange(val);
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    const { key, ctrlKey, metaKey, shiftKey } = e;

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

    // execute query
    if (enter && !shiftKey && !openAutocomplete) {
      onEnter();
    }
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
    />
    {autocomplete && (
      <Autocomplete
        value={value}
        options={options}
        anchor={autocompleteAnchorEl}
        onSelect={handleSelect}
        onOpenAutocomplete={setOpenAutocomplete}
      />
    )}
  </div>;
};

export default QueryEditor;
