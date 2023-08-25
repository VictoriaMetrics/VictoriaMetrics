import React, { FC, useEffect, useRef, useState } from "preact/compat";
import { ChangeEvent, KeyboardEvent } from "react";
import { CalendarIcon } from "../../Icons";
import DatePicker from "../DatePicker";
import Button from "../../Button/Button";
import { DATE_TIME_FORMAT } from "../../../../constants/date";
import InputMask from "react-input-mask";
import dayjs from "dayjs";
import classNames from "classnames";
import "./style.scss";

const formatStringDate = (val: string) => {
  return dayjs(val).isValid() ? dayjs.tz(val).format(DATE_TIME_FORMAT) : val;
};

interface DateTimeInputProps {
  value?:  string;
  label: string;
  pickerLabel: string;
  pickerRef: React.RefObject<HTMLDivElement>;
  onChange: (date: string) => void;
  onEnter: () => void;
}

const DateTimeInput: FC<DateTimeInputProps> = ({
  value = "",
  label,
  pickerLabel,
  pickerRef,
  onChange,
  onEnter
}) => {
  const wrapperRef = useRef<HTMLDivElement>(null);
  const [inputRef, setInputRef] = useState<HTMLInputElement | null>(null);

  const [maskedValue, setMaskedValue] = useState(formatStringDate(value));
  const [focusToTime, setFocusToTime] = useState(false);
  const [awaitChangeForEnter, setAwaitChangeForEnter] = useState(false);
  const error = dayjs(maskedValue).isValid() ? "" : "Invalid date format";

  const handleMaskedChange = (e: ChangeEvent<HTMLInputElement>) => {
    setMaskedValue(e.currentTarget.value);
  };

  const handleBlur = () => {
    onChange(maskedValue);
  };

  const handleKeyUp = (e: KeyboardEvent) => {
    if (e.key === "Enter") {
      onChange(maskedValue);
      setAwaitChangeForEnter(true);
    }
  };

  const handleChangeDate = (val: string) => {
    setMaskedValue(val);
    setFocusToTime(true);
  };

  useEffect(() => {
    const newValue = formatStringDate(value);
    if (newValue !== maskedValue) {
      setMaskedValue(newValue);
    }

    if (awaitChangeForEnter) {
      onEnter();
      setAwaitChangeForEnter(false);
    }
  }, [value]);

  useEffect(() => {
    if (focusToTime && inputRef) {
      inputRef.focus();
      inputRef.setSelectionRange(11, 11);
      setFocusToTime(false);
    }
  }, [focusToTime]);

  return (
    <div
      className={classNames({
        "vm-date-time-input": true,
        "vm-date-time-input_error": error
      })}
    >
      <label>{label}</label>
      <InputMask
        tabIndex={1}
        inputRef={setInputRef}
        mask="9999-99-99 99:99:99"
        placeholder="YYYY-MM-DD HH:mm:ss"
        value={maskedValue}
        autoCapitalize={"none"}
        inputMode={"numeric"}
        maskChar={null}
        onChange={handleMaskedChange}
        onBlur={handleBlur}
        onKeyUp={handleKeyUp}
      />
      {error && (
        <span className="vm-date-time-input__error-text">{error}</span>
      )}
      <div
        className="vm-date-time-input__icon"
        ref={wrapperRef}
      >
        <Button
          variant="text"
          color="gray"
          size="small"
          startIcon={<CalendarIcon/>}
          ariaLabel="calendar"
        />
      </div>
      <DatePicker
        label={pickerLabel}
        ref={pickerRef}
        date={maskedValue}
        onChange={handleChangeDate}
        targetRef={wrapperRef}
      />
    </div>
  );
};

export default DateTimeInput;
