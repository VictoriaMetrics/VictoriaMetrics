import React, { FC, Ref, useEffect, useMemo, useRef, useState } from "preact/compat";
import classNames from "classnames";
import useClickOutside from "../../../hooks/useClickOutside";
import Popper from "../Popper/Popper";
import "./style.scss";
import { DoneIcon } from "../Icons";

interface AutocompleteProps {
  value: string
  options: string[]
  anchor: Ref<HTMLElement>
  disabled?: boolean
  maxWords?: number
  minLength?: number
  fullWidth?: boolean
  noOptionsText?: string
  selected?: string[]
  onSelect: (val: string) => void,
  onOpenAutocomplete?: (val: boolean) => void
}

const Autocomplete: FC<AutocompleteProps> = ({
  value,
  options,
  anchor,
  disabled,
  maxWords = 1,
  minLength = 2,
  fullWidth,
  selected,
  noOptionsText,
  onSelect,
  onOpenAutocomplete
}) => {
  const wrapperEl = useRef<HTMLDivElement>(null);

  const [openAutocomplete, setOpenAutocomplete] = useState(false);
  const [focusOption, setFocusOption] = useState(-1);

  const foundOptions = useMemo(() => {
    if (!openAutocomplete) return [];
    try {
      const regexp = new RegExp(String(value), "i");
      const found = options.filter((item) => regexp.test(item) && (item !== value));
      return found.sort((a,b) => (a.match(regexp)?.index || 0) - (b.match(regexp)?.index || 0));
    } catch (e) {
      return [];
    }
  }, [openAutocomplete, options, value]);

  const displayNoOptionsText = useMemo(() => {
    return noOptionsText && !foundOptions.length;
  }, [noOptionsText,foundOptions]);

  const handleCloseAutocomplete = () => {
    setOpenAutocomplete(false);
  };

  const createHandlerSelect = (item: string) => () => {
    if (disabled) return;
    onSelect(item);
    if (!selected) handleCloseAutocomplete();
  };

  const scrollToValue = () => {
    if (!wrapperEl.current) return;
    const target = wrapperEl.current.childNodes[focusOption] as HTMLElement;
    if (target?.scrollIntoView) target.scrollIntoView({ block: "center" });
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    const { key, ctrlKey, metaKey, shiftKey } = e;
    const modifiers = ctrlKey || metaKey || shiftKey;
    const hasOptions = foundOptions.length;

    if (key === "ArrowUp" && !modifiers && hasOptions) {
      e.preventDefault();
      setFocusOption((prev) => prev <= 0 ? 0 : prev - 1);
    }

    if (key === "ArrowDown" && !modifiers && hasOptions) {
      e.preventDefault();
      const lastIndex = foundOptions.length - 1;
      setFocusOption((prev) => prev >= lastIndex ? lastIndex : prev + 1);
    }

    if (key === "Enter") {
      const value = foundOptions[focusOption];
      value && onSelect(value);
      if (!selected) handleCloseAutocomplete();
    }

    if (key === "Escape") {
      handleCloseAutocomplete();
    }
  };

  useEffect(() => {
    const words = (value.match(/[a-zA-Z_:.][a-zA-Z0-9_:.]*/gm) || []).length;
    setOpenAutocomplete(value.length > minLength && words <= maxWords);
  }, [value]);

  useEffect(() => {
    scrollToValue();

    window.addEventListener("keydown", handleKeyDown);

    return () => {
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [focusOption, foundOptions]);

  useEffect(() => {
    setFocusOption(-1);
  }, [foundOptions]);

  useEffect(() => {
    onOpenAutocomplete && onOpenAutocomplete(openAutocomplete);
  }, [openAutocomplete]);

  useClickOutside(wrapperEl, handleCloseAutocomplete, anchor);

  return (
    <Popper
      open={openAutocomplete}
      buttonRef={anchor}
      placement="bottom-left"
      onClose={handleCloseAutocomplete}
      fullWidth={fullWidth}
    >
      <div
        className="vm-autocomplete"
        ref={wrapperEl}
      >
        {displayNoOptionsText && <div className="vm-autocomplete__no-options">{noOptionsText}</div>}
        {foundOptions.map((option, i) =>
          <div
            className={classNames({
              "vm-list-item": true,
              "vm-list-item_active": i === focusOption,
              "vm-list-item_multiselect": selected,
              "vm-list-item_multiselect_selected": selected?.includes(option)
            })}
            id={`$autocomplete$${option}`}
            key={option}
            onClick={createHandlerSelect(option)}
          >
            {selected?.includes(option) && <DoneIcon/>}
            <span>{option}</span>
          </div>
        )}
      </div>
    </Popper>
  );
};

export default Autocomplete;
