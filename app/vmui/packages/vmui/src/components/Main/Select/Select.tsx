import React, { FC, useEffect, useMemo, useRef, useState,  } from "preact/compat";
import classNames from "classnames";
import { ArrowDropDownIcon, CloseIcon } from "../Icons";
import { FormEvent, MouseEvent } from "react";
import Autocomplete from "../Autocomplete/Autocomplete";
import { useAppState } from "../../../state/common/StateContext";
import "./style.scss";
import useDeviceDetect from "../../../hooks/useDeviceDetect";

interface SelectProps {
  value: string | string[]
  list: string[]
  label?: string
  placeholder?: string
  noOptionsText?: string
  clearable?: boolean
  searchable?: boolean
  autofocus?: boolean
  onChange: (value: string) => void
}

const Select: FC<SelectProps> = ({
  value,
  list,
  label,
  placeholder,
  noOptionsText,
  clearable = false,
  searchable = false,
  autofocus,
  onChange
}) => {
  const { isDarkTheme } = useAppState();
  const { isMobile } = useDeviceDetect();

  const [search, setSearch] = useState("");
  const autocompleteAnchorEl = useRef<HTMLDivElement>(null);
  const [openList, setOpenList] = useState(false);

  const inputRef = useRef<HTMLInputElement>(null);

  const isMultiple = useMemo(() => Array.isArray(value), [value]);
  const selectedValues = useMemo(() => Array.isArray(value) ? value : undefined, [isMultiple, value]);

  const textFieldValue = useMemo(() => {
    if (openList) return search;
    return Array.isArray(value) ? "" : value;
  }, [value, search, openList, isMultiple]);

  const autocompleteValue = useMemo(() => !openList ? "" : search || "(.+)", [search, openList]);

  const clearFocus = () => {
    if (inputRef.current) {
      inputRef.current.blur();
    }
  };

  const handleCloseList = () => {
    setOpenList(false);
    clearFocus();
  };

  const handleFocus = () => {
    setOpenList(true);
  };

  const handleToggleList = (e: MouseEvent<HTMLDivElement>) => {
    if (e.target instanceof HTMLInputElement) return;
    setOpenList(prev => !prev);
  };

  const handleSelected = (val: string) => {
    onChange(val);
    if (!isMultiple) handleCloseList();
    if (isMultiple && inputRef.current) inputRef.current.focus();
  };

  const handleChange = (e: FormEvent<HTMLInputElement>) => {
    setSearch((e.target as HTMLInputElement).value);
  };

  const createHandleClick = (value: string) => (e: MouseEvent) => {
    handleSelected(value);
    e.stopPropagation();
  };

  const handleKeyUp = (e: KeyboardEvent) => {
    if (inputRef.current !== e.target) {
      setOpenList(false);
    }
  };

  useEffect(() => {
    setSearch("");
    if (openList && inputRef.current) {
      inputRef.current.focus();
    }
    if (!openList) clearFocus();
  }, [openList, inputRef]);

  useEffect(() => {
    if (!autofocus || !inputRef.current || isMobile) return;
    inputRef.current.focus();
  }, [autofocus, inputRef]);

  useEffect(() => {
    window.addEventListener("keyup", handleKeyUp);

    return () => {
      window.removeEventListener("keyup", handleKeyUp);
    };
  }, []);

  return (
    <div
      className={classNames({
        "vm-select": true,
        "vm-select_dark": isDarkTheme
      })}
    >
      <div
        className="vm-select-input"
        onClick={handleToggleList}
        ref={autocompleteAnchorEl}
      >
        <div className="vm-select-input-content">
          {!isMobile && selectedValues && selectedValues.map(item => (
            <div
              className="vm-select-input-content__selected"
              key={item}
            >
              <span>{item}</span>
              <div onClick={createHandleClick(item)}>
                <CloseIcon/>
              </div>
            </div>
          ))}
          {isMobile && !!selectedValues?.length && (
            <span className="vm-select-input-content__counter">
              selected {selectedValues.length}
            </span>
          )}
          {(!isMobile || (isMobile && !selectedValues?.length)) && (
            <input
              value={textFieldValue}
              type="text"
              placeholder={placeholder}
              onInput={handleChange}
              onFocus={handleFocus}
              ref={inputRef}
              readOnly={isMobile || !searchable}
            />
          )}
        </div>
        {label && <span className="vm-text-field__label">{label}</span>}
        {clearable && value && (
          <div
            className="vm-select-input__icon"
            onClick={createHandleClick("")}
          >
            <CloseIcon/>
          </div>
        )}
        <div
          className={classNames({
            "vm-select-input__icon": true,
            "vm-select-input__icon_open": openList
          })}
        >
          <ArrowDropDownIcon/>
        </div>
      </div>
      <Autocomplete
        label={label}
        value={autocompleteValue}
        options={list}
        anchor={autocompleteAnchorEl}
        selected={selectedValues}
        maxWords={10}
        minLength={0}
        fullWidth
        noOptionsText={noOptionsText}
        onSelect={handleSelected}
        onOpenAutocomplete={setOpenList}
      />
    </div>
  );
};

export default Select;
