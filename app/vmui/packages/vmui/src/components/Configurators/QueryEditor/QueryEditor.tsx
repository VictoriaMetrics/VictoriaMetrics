import React, { FC, useEffect, useMemo, useRef, useState } from "preact/compat";
import { KeyboardEvent } from "react";
import { ErrorTypes } from "../../../types";
import Popper from "@mui/material/Popper";
import TextField from "@mui/material/TextField";
import Box from "@mui/material/Box";
import Paper from "@mui/material/Paper";
import MenuItem from "@mui/material/MenuItem";
import MenuList from "@mui/material/MenuList";
import ClickAwayListener from "@mui/material/ClickAwayListener";

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
  size = "medium"
}) => {

  const [focusOption, setFocusOption] = useState(-1);
  const [openAutocomplete, setOpenAutocomplete] = useState(false);

  const autocompleteAnchorEl = useRef<HTMLDivElement>(null);
  const wrapperEl = useRef<HTMLUListElement>(null);

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

  const handleKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
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
    } else if (enter && !shiftKey) {
      onEnter();
    }
  };

  useEffect(() => {
    if (!wrapperEl.current) return;
    const target = wrapperEl.current.childNodes[focusOption] as HTMLElement;
    if (target?.scrollIntoView) target.scrollIntoView({ block: "center" });
  }, [focusOption]);

  return <Box ref={autocompleteAnchorEl}>
    <TextField
      defaultValue={value}
      fullWidth
      label={label}
      multiline
      focused={!!value}
      error={!!error}
      onKeyDown={handleKeyDown}
      onChange={(e) => onChange(e.target.value)}
      size={size}
    />
    <Popper
      open={openAutocomplete}
      anchorEl={autocompleteAnchorEl.current}
      placement="bottom-start"
      sx={{ zIndex: 3 }}
    >
      <ClickAwayListener onClickAway={() => setOpenAutocomplete(false)}>
        <Paper
          elevation={3}
          sx={{ maxHeight: 300, overflow: "auto" }}
        >
          <MenuList
            ref={wrapperEl}
            dense
          >
            {foundOptions.map((item, i) =>
              <MenuItem
                id={`$autocomplete$${item}`}
                key={item}
                sx={{ bgcolor: `rgba(0, 0, 0, ${i === focusOption ? 0.12 : 0})` }}
                onClick={() => {
                  onChange(item);
                  setOpenAutocomplete(false);
                }}
              >
                {item}
              </MenuItem>)}
          </MenuList>
        </Paper>
      </ClickAwayListener>
    </Popper>
  </Box>;
};

export default QueryEditor;
