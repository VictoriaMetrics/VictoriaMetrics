import { FC, useEffect, useRef, useState } from "preact/compat";
import { ChangeEvent, KeyboardEvent } from "react";
import { CalendarIcon } from "../../Icons";
import DatePicker from "../DatePicker";
import Button from "../../Button/Button";
import { DATE_ISO_FORMAT, DATE_FORMAT, DATE_TIME_FORMAT } from "../../../../constants/date";
import InputMask from "react-input-mask";
import dayjs, { Dayjs } from "dayjs";
import classNames from "classnames";
import "./style.scss";

const formatStringDate = (val: string, format: string) => {
  return dayjs(val).isValid() ? dayjs.tz(val).format(format) : val;
};

interface DateTimeInputProps {
  value?:  string;
  label: string;
  pickerLabel: string;
  format?: string;
  pickerRef: React.RefObject<HTMLDivElement>;
  onChange: (date: string) => void;
  onEnter: () => void;
  disabled?: boolean;
  minDate?: Date | Dayjs;
  maxDate?: Date | Dayjs;
}

const masks: Record<string, string> = {
  [DATE_ISO_FORMAT]: "9999-99-99T99:99:99",
  [DATE_FORMAT]: "9999-99-99",
  [DATE_TIME_FORMAT]: "9999-99-99 99:99:99"
};

const DateTimeInput: FC<DateTimeInputProps> = ({
  value = "",
  format = DATE_TIME_FORMAT,
  minDate,
  maxDate,
  label,
  pickerLabel,
  pickerRef,
  onChange,
  onEnter,
  disabled
}) => {
  const wrapperRef = useRef<HTMLDivElement>(null);
  const [inputRef, setInputRef] = useState<HTMLInputElement | null>(null);
  const mask = masks[format];

  const [maskedValue, setMaskedValue] = useState(formatStringDate(value, format));
  const [focusToTime, setFocusToTime] = useState(false);
  const [awaitChangeForEnter, setAwaitChangeForEnter] = useState(false);
  const error = dayjs(maskedValue).isValid() ? "" : "Invalid date format";

  const handleMaskedChange = (e: ChangeEvent<HTMLInputElement>) => {
    setMaskedValue(e.currentTarget.value);
  };

  const handleBlur = () => {
    onChange(maskedValue);
  };

  const handleKeyUp = (e: KeyboardEvent<HTMLInputElement>) => {
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
    const newValue = formatStringDate(value, format);
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
        "vm-date-time-input_error": error,
        "vm-date-time-input_disabled": disabled,
      })}
    >
      <label>{label}</label>
      <InputMask
        tabIndex={1}
        inputRef={setInputRef}
        mask={mask}
        placeholder={format}
        value={maskedValue}
        autoCapitalize={"none"}
        inputMode={"numeric"}
        maskChar={null}
        onChange={handleMaskedChange}
        onBlur={handleBlur}
        onKeyUp={handleKeyUp}
        disabled={disabled}
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
          disabled={disabled}
        />
      </div>
      <DatePicker
        label={pickerLabel}
        ref={pickerRef}
        date={maskedValue}
        onChange={handleChangeDate}
        targetRef={wrapperRef}
        minDate={minDate}
        maxDate={maxDate}
        format={format}
      />
    </div>
  );
};

export default DateTimeInput;
