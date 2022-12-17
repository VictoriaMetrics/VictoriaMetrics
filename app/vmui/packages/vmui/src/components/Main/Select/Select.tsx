import React, { FC, useEffect, useMemo, useRef, useState } from "preact/compat";
import classNames from "classnames";
import { ArrowDropDownIcon, CloseIcon } from "../Icons";
import TextField from "../../../components/Main/TextField/TextField";
import { MouseEvent } from "react";
import Autocomplete from "../Autocomplete/Autocomplete";
import "./style.scss";

interface JobSelectorProps {
  value: string
  list: string[]
  label?: string
  placeholder?: string
  noOptionsText?: string
  error?: string
  clearable?: boolean
  searchable?: boolean
  onChange: (value: string) => void
}

const Select: FC<JobSelectorProps> = ({
  value,
  list,
  label,
  placeholder,
  error,
  noOptionsText,
  clearable = false,
  searchable,
  onChange
}) => {

  const [search, setSearch] = useState("");
  const autocompleteAnchorEl = useRef<HTMLDivElement>(null);
  const [openList, setOpenList] = useState(false);

  const textFieldValue = useMemo(() => openList ? search : value, [value, search, openList]);
  const autocompleteValue = useMemo(() => !openList ? "" : search || "(.+)", [search, openList]);

  const clearFocus = () => {
    if (document.activeElement instanceof HTMLInputElement) {
      document.activeElement.blur();
    }
  };

  const handleCloseList = () => {
    setOpenList(false);
    clearFocus();
  };

  const handleFocus = () => {
    setOpenList(true);
  };

  const handleClickJob = (job: string) => {
    onChange(job);
    handleCloseList();
  };

  const createHandleClick = (job: string) => (e: MouseEvent<HTMLDivElement>) => {
    handleClickJob(job);
    e.stopPropagation();
  };

  useEffect(() => {
    setSearch("");
  }, [openList]);

  return (
    <div className="vm-select">
      <div
        className="vm-select-input"
        ref={autocompleteAnchorEl}
      >
        <TextField
          label={label}
          type="text"
          value={textFieldValue}
          placeholder={placeholder}
          error={error}
          disabled={!searchable}
          onFocus={handleFocus}
          onEnter={handleCloseList}
          onChange={setSearch}
          endIcon={(
            <div
              className={classNames({
                "vm-select-input__icon": true,
                "vm-select-input__icon_open": openList
              })}
            >
              <ArrowDropDownIcon/>
            </div>
          )}
        />
        {clearable && (
          <div
            className="vm-select-input__clear"
            onClick={createHandleClick("")}
          >
            <CloseIcon/>
          </div>
        )}
      </div>
      <Autocomplete
        value={autocompleteValue}
        options={list}
        anchor={autocompleteAnchorEl}
        maxWords={10}
        minLength={0}
        fullWidth
        noOptionsText={noOptionsText}
        onSelect={handleClickJob}
        onOpenAutocomplete={setOpenList}
      />
    </div>
  );
};

export default Select;
