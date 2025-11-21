import { FC, useEffect, useMemo, useRef, useState, } from "preact/compat";
import classNames from "classnames";
import { ArrowDropDownIcon, CloseIcon } from "../Icons";
import { FormEvent, MouseEvent } from "react";
import Autocomplete from "../Autocomplete/Autocomplete";
import { useAppState } from "../../../state/common/StateContext";
import "./style.scss";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import MultipleSelectedValue from "./MultipleSelectedValue/MultipleSelectedValue";
import useEventListener from "../../../hooks/useEventListener";
import useClickOutside from "../../../hooks/useClickOutside";

interface SelectProps {
  itemClassName?: string
  value: string | string[]
  list: string[]
  label?: string
  placeholder?: string
  noOptionsText?: string
  clearable?: boolean
  searchable?: boolean
  autofocus?: boolean
  disabled?: boolean
  includeAll?: boolean
  onChange: (value: string) => void
}

const Select: FC<SelectProps> = ({
  value,
  list,
  label,
  itemClassName,
  placeholder,
  noOptionsText,
  clearable = false,
  searchable = false,
  autofocus,
  disabled,
  includeAll,
  onChange
}) => {
  const { isDarkTheme } = useAppState();
  const { isMobile } = useDeviceDetect();

  const [search, setSearch] = useState("");
  const autocompleteAnchorEl = useRef<HTMLDivElement>(null);
  const [wrapperRef, setWrapperRef] = useState<React.RefObject<HTMLElement> | null>(null);
  const [openList, setOpenList] = useState(false);
  const resultList = [...list];

  const inputRef = useRef<HTMLInputElement>(null);

  const isMultiple = Array.isArray(value);
  let selectedValues = Array.isArray(value) ? value.slice() : [];
  const hideInput = isMobile && isMultiple && !!selectedValues?.length;

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
    if (disabled) return;
    setOpenList(true);
  };

  const handleBlur = () => {
    resultList.includes(search) && onChange(search);
  };

  const handleToggleList = (e: MouseEvent<HTMLDivElement>) => {
    if (e.target instanceof HTMLInputElement || disabled) return;
    setOpenList(prev => !prev);
  };

  const handleSelected = (val: string) => {
    setSearch("");
    onChange(val);
    if (!isMultiple) handleCloseList();
    if (isMultiple && inputRef.current) inputRef.current.focus();
  };

  const handleChange = (e: FormEvent<HTMLInputElement>) => {
    setSearch((e.target as HTMLInputElement).value);
  };

  const createHandleClick = (value: string) => (e: MouseEvent<HTMLDivElement>) => {
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

  useEventListener("keyup", handleKeyUp);
  useClickOutside(autocompleteAnchorEl, handleCloseList, wrapperRef);

  if (includeAll && !resultList.includes("All")) resultList.push("All");
  if (includeAll && (!selectedValues?.length || selectedValues?.length === resultList?.length)) {
    selectedValues = ["All"];
  }

  return (
    <div
      className={classNames({
        "vm-select": true,
        "vm-select_dark": isDarkTheme,
        "vm-select_disabled": disabled
      })}
    >
      {label && <span className="vm-text-field__label">{label}</span>}
      <div
        className="vm-select-input"
        onClick={handleToggleList}
        ref={autocompleteAnchorEl}
      >
        <div className="vm-select-input-content">
          {!!selectedValues?.length && (
            <MultipleSelectedValue
              itemClassName={itemClassName}
              values={selectedValues}
              onRemoveItem={handleSelected}
            />
          )}
          {!hideInput && (
            <input
              value={textFieldValue}
              type="text"
              placeholder={placeholder}
              onInput={handleChange}
              onFocus={handleFocus}
              onBlur={handleBlur}
              disabled={disabled}
              ref={inputRef}
              readOnly={isMobile || !searchable}
            />
          )}
        </div>
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
        itemClassName={itemClassName}
        label={label}
        value={autocompleteValue}
        options={resultList.map(l => ({ value: l }))}
        anchor={autocompleteAnchorEl}
        selected={selectedValues}
        minLength={1}
        fullWidth
        noOptionsText={noOptionsText}
        onSelect={handleSelected}
        onOpenAutocomplete={setOpenList}
        onChangeWrapperRef={setWrapperRef}
      />
    </div>
  );
};

export default Select;
