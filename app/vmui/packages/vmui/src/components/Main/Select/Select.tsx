import React, { FC, useRef, useState } from "preact/compat";
import classNames from "classnames";
import { ArrowDropDownIcon, CloseIcon } from "../Icons";
import Popper from "../../../components/Main/Popper/Popper";
import TextField from "../../../components/Main/TextField/TextField";
import "./style.scss";
import { MouseEvent } from "react";

interface JobSelectorProps {
  value: string
  list: string[]
  label?: string
  placeholder?: string
  noOptionsText?: string
  error?: string
  clearable?: boolean
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
  onChange
}) => {

  const [openList, setOpenList] = useState(false);
  const targetRef = useRef<HTMLDivElement>(null);

  const toggleOpenList = () => {
    setOpenList(prev => !prev);
  };

  const handleCloseList = () => {
    setOpenList(false);
  };

  const handleClickJob = (job: string) => {
    onChange(job);
    handleCloseList();
  };

  const createHandleClick = (job: string) => (e: MouseEvent<HTMLDivElement>) => {
    handleClickJob(job);
    e.stopPropagation();
  };

  return (
    <div className="vm-select">
      <div
        className="vm-select-input"
        onClick={toggleOpenList}
        ref={targetRef}
      >
        <TextField
          label={label}
          type="text"
          value={value}
          placeholder={placeholder}
          error={error}
          disabled
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

      <Popper
        open={openList}
        buttonRef={targetRef}
        placement="bottom-left"
        fullWidth
        onClose={handleCloseList}
      >
        <div className="vm-select-list">
          {!list.length && <div className="vm-select-list__no-options">{noOptionsText || "No options"}</div>}
          {list.map(item => (
            <div
              className="vm-list__item"
              key={item}
              onClick={createHandleClick(item)}
            >
              {item}
            </div>
          ))}
        </div>
      </Popper>
    </div>
  );
};

export default Select;
