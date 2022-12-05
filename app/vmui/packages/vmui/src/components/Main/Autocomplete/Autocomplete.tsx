import React, { FC, Ref, useEffect, useMemo, useRef, useState } from "preact/compat";
import classNames from "classnames";
import useClickOutside from "../../../hooks/useClickOutside";
import Popper from "../Popper/Popper";
import "./style.scss";

interface AutocompleteProps {
  value: string
  options: string[]
  anchor: Ref<HTMLElement>
  disabled?: boolean
  maxWords?: number
  onSelect: (val: string) => void
}

const Autocomplete: FC<AutocompleteProps> = ({
  value,
  options,
  anchor,
  disabled,
  maxWords = 1,
  onSelect,
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

  const handleCloseAutocomplete = () => {
    setOpenAutocomplete(false);
  };

  const createHandlerSelect = (item: string) => () => {
    if (disabled) return;
    onSelect(item);
    handleCloseAutocomplete();
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
      handleCloseAutocomplete();
    }

    if (key === "Escape") {
      handleCloseAutocomplete();
    }
  };

  useEffect(() => {
    const words = (value.match(/[a-zA-Z_:.][a-zA-Z0-9_:.]*/gm) || []).length;
    setOpenAutocomplete(value.length > 2 && words <= maxWords);
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

  useClickOutside(wrapperEl, handleCloseAutocomplete);

  return (
    <Popper
      open={openAutocomplete}
      buttonRef={anchor}
      placement="bottom-left"
      onClose={handleCloseAutocomplete}
    >
      <div
        className="vm-autocomplete"
        ref={wrapperEl}
      >
        {foundOptions.map((option, i) =>
          <div
            className={classNames({
              "vm-list__item": true,
              "vm-list__item_active": i === focusOption
            })}
            id={`$autocomplete$${option}`}
            key={option}
            onClick={createHandlerSelect(option)}
          >
            {option}
          </div>
        )}
      </div>
    </Popper>
  );
};

export default Autocomplete;
