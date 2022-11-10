import React, { FC, useEffect, useMemo, useRef, useState } from "preact/compat";
import { KeyboardEvent } from "react";
import { ErrorTypes } from "../../../types";
import TextField from "../../Main/TextField/TextField";
import Popper from "../../Main/Popper/Popper";
import useClickOutside from "../../../hooks/useClickOutside";
import "./style.scss";
import classNames from "classnames";

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
  size?: "small" | "medium" | undefined;
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
}) => {

  const [focusOption, setFocusOption] = useState(-1);
  const [openAutocomplete, setOpenAutocomplete] = useState(false);

  const autocompleteAnchorEl = useRef<HTMLDivElement>(null);
  const wrapperEl = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const words = (value.match(/[a-zA-Z_:.][a-zA-Z0-9_:.]*/gm) || []).length;
    setOpenAutocomplete(autocomplete && value.length > 2 && words <= 1);
  }, [autocomplete, value]);

  const foundOptions = useMemo(() => {
    setFocusOption(0);
    if (!openAutocomplete) return [];
    try {
      const regexp = new RegExp(String(value), "i");
      const found = options.filter((item) => regexp.test(item) && (item !== value));
      return found.sort((a,b) => (a.match(regexp)?.index || 0) - (b.match(regexp)?.index || 0));
    } catch (e) {
      return [];
    }
  }, [openAutocomplete, options]);

  const handleKeyDown = (e: KeyboardEvent) => {
    const { key, ctrlKey, metaKey, shiftKey } = e;

    const ctrlMetaKey = ctrlKey || metaKey;
    const arrowUp = key === "ArrowUp";
    const arrowDown = key === "ArrowDown";
    const enter = key === "Enter";

    const hasAutocomplete = openAutocomplete && foundOptions.length;

    const isArrows = arrowUp || arrowDown;
    const arrowsByOptions = isArrows && hasAutocomplete;
    const arrowsByHistory = isArrows && ctrlMetaKey;
    const enterByOptions = enter && hasAutocomplete;

    if (arrowsByOptions || arrowsByHistory || enterByOptions) {
      e.preventDefault();
    }

    // ArrowUp
    if (arrowUp && hasAutocomplete && !ctrlMetaKey) {
      setFocusOption((prev) => prev === 0 ? 0 : prev - 1);
    } else if (arrowUp && ctrlMetaKey) {
      onArrowUp();
    }

    // ArrowDown
    if (arrowDown && hasAutocomplete && !ctrlMetaKey) {
      setFocusOption((prev) => prev >= foundOptions.length - 1 ? foundOptions.length - 1 : prev + 1);
    } else if (arrowDown && ctrlMetaKey) {
      onArrowDown();
    }

    // Enter
    if (enter && hasAutocomplete && !shiftKey && !ctrlMetaKey) {
      onChange(foundOptions[focusOption]);
      setOpenAutocomplete(false);
    } else if (enter && !shiftKey) {
      onEnter();
    }
  };

  useEffect(() => {
    if (!wrapperEl.current) return;
    const target = wrapperEl.current.childNodes[focusOption] as HTMLElement;
    if (target?.scrollIntoView) target.scrollIntoView({ block: "center" });
  }, [focusOption]);

  useClickOutside(autocompleteAnchorEl, () => setOpenAutocomplete(false), wrapperEl);

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
      // size={size}
    />
    <Popper
      open={openAutocomplete}
      buttonRef={autocompleteAnchorEl}
      placement="bottom-left"
      onClose={() => setOpenAutocomplete(false)}
    >
      <div
        className="vm-query-editor-autocomplete"
        ref={wrapperEl}
      >
        {foundOptions.map((item, i) =>
          <div
            className={classNames({
              "vm-list__item": true,
              "vm-list__item_active": i === focusOption
            })}
            id={`$autocomplete$${item}`}
            key={item}
            onClick={() => {
              onChange(item);
              setOpenAutocomplete(false);
            }}
          >
            {item}
          </div>)}
      </div>
    </Popper>
  </div>;
};

export default QueryEditor;
