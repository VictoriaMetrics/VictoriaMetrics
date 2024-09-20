import React, { FC, useCallback, useEffect, useMemo, useRef, useState, JSX } from "preact/compat";
import classNames from "classnames";
import Popper from "../Popper/Popper";
import "./style.scss";
import { DoneIcon, RefreshIcon } from "../Icons";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import useBoolean from "../../../hooks/useBoolean";
import useEventListener from "../../../hooks/useEventListener";

export interface AutocompleteOptions {
  value: string;
  description?: string;
  type?: string;
  icon?: JSX.Element
}

interface AutocompleteProps {
  value: string
  options: AutocompleteOptions[]
  anchor: React.RefObject<HTMLElement>
  disabled?: boolean
  minLength?: number
  fullWidth?: boolean
  noOptionsText?: string
  selected?: string[]
  label?: string
  disabledFullScreen?: boolean
  offset?: {top: number, left: number}
  maxDisplayResults?: {limit: number, message?: string}
  loading?: boolean;
  onSelect: (val: string) => void
  onOpenAutocomplete?: (val: boolean) => void
  onFoundOptions?: (val: AutocompleteOptions[]) => void
  onChangeWrapperRef?: (elementRef: React.RefObject<HTMLElement>) => void
}

enum FocusType {
  mouse,
  keyboard
}

const Autocomplete: FC<AutocompleteProps> = ({
  value,
  options,
  anchor,
  disabled,
  minLength = 2,
  fullWidth,
  selected,
  noOptionsText,
  label,
  disabledFullScreen,
  offset,
  maxDisplayResults,
  loading,
  onSelect,
  onOpenAutocomplete,
  onFoundOptions,
  onChangeWrapperRef
}) => {
  const { isMobile } = useDeviceDetect();
  const wrapperEl = useRef<HTMLDivElement>(null);

  const [focusOption, setFocusOption] = useState<{index: number, type?: FocusType}>({ index: -1 });
  const [showMessage, setShowMessage] = useState("");
  const [totalFound, setTotalFound] = useState(0);

  const {
    value: openAutocomplete,
    setValue: setOpenAutocomplete,
    setFalse: handleCloseAutocomplete,
  } = useBoolean(false);

  const foundOptions = useMemo(() => {
    if (!openAutocomplete) return [];
    try {
      const regexp = new RegExp(String(value.trim()), "i");
      const found = options.filter((item) => regexp.test(item.value));
      const sorted = found.sort((a, b) => {
        if (a.value.toLowerCase() === value.trim().toLowerCase()) return -1;
        if (b.value.toLowerCase() === value.trim().toLowerCase()) return 1;
        return (a.value.match(regexp)?.index || 0) - (b.value.match(regexp)?.index || 0);
      });
      setTotalFound(sorted.length);
      setShowMessage(sorted.length > Number(maxDisplayResults?.limit) ? maxDisplayResults?.message || "" : "");
      return maxDisplayResults?.limit ? sorted.slice(0, maxDisplayResults.limit) : sorted;
    } catch (e) {
      return [];
    }
  }, [openAutocomplete, options, value]);

  const hideFoundedOptions = useMemo(() => {
    return foundOptions.length === 1 && foundOptions[0]?.value === value;
  }, [foundOptions]);

  const displayNoOptionsText = useMemo(() => {
    return noOptionsText && !foundOptions.length;
  }, [noOptionsText,foundOptions]);

  const createHandlerSelect = (item: string) => () => {
    if (disabled) return;
    onSelect(item);
    if (!selected) handleCloseAutocomplete();
  };

  const createHandlerMouseEnter = (index: number) => () => {
    setFocusOption({ index, type: FocusType.mouse });
  };

  const handlerMouseLeave = () => {
    setFocusOption({ index: -1 });
  };

  const scrollToValue = () => {
    if (!wrapperEl.current || focusOption.type === FocusType.mouse) return;
    const target = wrapperEl.current.childNodes[focusOption.index] as HTMLElement;
    if (target?.scrollIntoView) target.scrollIntoView({ block: "center" });
  };

  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    const { key, ctrlKey, metaKey, shiftKey } = e;
    const modifiers = ctrlKey || metaKey || shiftKey;
    const hasOptions = foundOptions.length && !hideFoundedOptions;

    if (key === "ArrowUp" && !modifiers && hasOptions) {
      e.preventDefault();
      setFocusOption(({ index }) => ({
        index:  index <= 0 ? 0 : index - 1,
        type: FocusType.keyboard
      }));
    }

    if (key === "ArrowDown" && !modifiers && hasOptions) {
      e.preventDefault();
      const lastIndex = foundOptions.length - 1;
      setFocusOption(({ index }) => ({
        index: index >= lastIndex ? lastIndex : index + 1,
        type: FocusType.keyboard
      }));
    }

    if (key === "Enter") {
      const item = foundOptions[focusOption.index];
      item && onSelect(item.value);
      if (!selected) handleCloseAutocomplete();
    }

    if (key === "Escape") {
      handleCloseAutocomplete();
    }
  }, [focusOption, foundOptions, hideFoundedOptions, handleCloseAutocomplete, onSelect, selected]);

  useEffect(() => {
    setOpenAutocomplete(value.length >= minLength);
  }, [value, options]);

  useEventListener("keydown", handleKeyDown);

  useEffect(scrollToValue, [focusOption, foundOptions]);

  useEffect(() => {
    setFocusOption({ index: -1 });
  }, [foundOptions]);

  useEffect(() => {
    onOpenAutocomplete && onOpenAutocomplete(openAutocomplete);
  }, [openAutocomplete]);

  useEffect(() => {
    onFoundOptions && onFoundOptions(hideFoundedOptions ? [] : foundOptions);
  }, [foundOptions, hideFoundedOptions]);

  useEffect(() => {
    onChangeWrapperRef && onChangeWrapperRef(wrapperEl);
  }, [wrapperEl]);

  return (
    <Popper
      open={openAutocomplete}
      buttonRef={anchor}
      placement="bottom-left"
      onClose={handleCloseAutocomplete}
      fullWidth={fullWidth}
      title={isMobile ? label : undefined}
      disabledFullScreen={disabledFullScreen}
      offset={offset}
    >
      <div
        className={classNames({
          "vm-autocomplete": true,
          "vm-autocomplete_mobile": isMobile && !disabledFullScreen,
        })}
        ref={wrapperEl}
      >
        {loading && <div className="vm-autocomplete__loader"><RefreshIcon/><span>Loading...</span></div>}
        {displayNoOptionsText && <div className="vm-autocomplete__no-options">{noOptionsText}</div>}
        {!hideFoundedOptions && foundOptions.map((option, i) =>
          <div
            className={classNames({
              "vm-list-item": true,
              "vm-list-item_mobile": isMobile,
              "vm-list-item_active": i === focusOption.index,
              "vm-list-item_multiselect": selected,
              "vm-list-item_multiselect_selected": selected?.includes(option.value),
              "vm-list-item_with-icon":  option.icon,
            })}
            id={`$autocomplete$${option.value}`}
            key={`${i}${option.value}`}
            onClick={createHandlerSelect(option.value)}
            onMouseEnter={createHandlerMouseEnter(i)}
            onMouseLeave={handlerMouseLeave}
          >
            {selected?.includes(option.value) && <DoneIcon/>}
            <>{option.icon}</>
            <span>{option.value}</span>
          </div>
        )}
      </div>
      {showMessage && (
        <div className="vm-autocomplete-message">
          Shown {maxDisplayResults?.limit} results out of {totalFound}. {showMessage}
        </div>
      )}
      {foundOptions[focusOption.index]?.description && (
        <div className="vm-autocomplete-info">
          <div className="vm-autocomplete-info__type">
            {foundOptions[focusOption.index].type}
          </div>
          <div
            className="vm-autocomplete-info__description"
            dangerouslySetInnerHTML={{ __html: foundOptions[focusOption.index].description || "" }}
          />
        </div>
      )}
    </Popper>
  );
};

export default Autocomplete;
