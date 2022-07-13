import React, {FC, useEffect, useMemo, useRef, useState} from "preact/compat";
import {KeyboardEvent} from "react";
import {ErrorTypes} from "../../../../types";
import Popper from "@mui/material/Popper";
import TextField from "@mui/material/TextField";
import Box from "@mui/material/Box";
import Paper from "@mui/material/Paper";
import MenuItem from "@mui/material/MenuItem";
import MenuList from "@mui/material/MenuList";
import ClickAwayListener from "@mui/material/ClickAwayListener";

export interface QueryEditorProps {
  setHistoryIndex: (step: number, index: number) => void;
  setQuery: (query: string, index: number) => void;
  runQuery: () => void;
  query: string;
  index: number;
  oneLiner?: boolean;
  autocomplete: boolean;
  error?: ErrorTypes | string;
  queryOptions: string[];
  label: string;
}

const QueryEditor: FC<QueryEditorProps> = ({
  index,
  query,
  setHistoryIndex,
  setQuery,
  runQuery,
  autocomplete,
  error,
  queryOptions,
  label,
}) => {

  const [focusField, setFocusField] = useState(false);
  const [focusOption, setFocusOption] = useState(-1);
  const autocompleteAnchorEl = useRef<HTMLDivElement>(null);
  const wrapperEl = useRef<HTMLUListElement>(null);
  const [openAutocomplete, setOpenAutocomplete] = useState(false);

  useEffect(() => {
    if (!focusField) return;
    const words = (query.match(/[a-zA-Z_:.][a-zA-Z0-9_:.]*/gm) || []).length;
    setOpenAutocomplete(!(!autocomplete || query.length < 2 || words > 1));
  },
  [autocomplete, query]);

  const actualOptions = useMemo(() => {
    setFocusOption(0);
    if (!openAutocomplete) return [];
    try {
      const regexp = new RegExp(String(query), "i");
      const options = queryOptions.filter((item) => regexp.test(item) && (item !== query));
      return options.sort((a,b) => (a.match(regexp)?.index || 0) - (b.match(regexp)?.index || 0));
    } catch (e) {
      return [];
    }
  }, [autocomplete, query, queryOptions]);

  const handleKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const {key, ctrlKey, metaKey, shiftKey} = e;

    const ctrlMetaKey = ctrlKey || metaKey;
    const arrowUp = key === "ArrowUp";
    const arrowDown = key === "ArrowDown";
    const enter = key === "Enter";

    const hasAutocomplete = openAutocomplete && actualOptions.length;

    if (((arrowUp || arrowDown) && (hasAutocomplete || ctrlMetaKey)) || (enter && (hasAutocomplete || ctrlMetaKey || !shiftKey))) {
      e.preventDefault();
    }

    // ArrowUp
    if (arrowUp && hasAutocomplete && !ctrlMetaKey) {
      setFocusOption((prev) => prev === 0 ? 0 : prev - 1);
    } else if (arrowUp && ctrlMetaKey) {
      setHistoryIndex(-1, index);
    }

    // ArrowDown
    if (arrowDown && hasAutocomplete && !ctrlMetaKey) {
      setFocusOption((prev) => prev >= actualOptions.length - 1 ? actualOptions.length - 1 : prev + 1);
    } else if (arrowDown && ctrlMetaKey) {
      setHistoryIndex(1, index);
    }

    // Enter
    if (enter && hasAutocomplete && !shiftKey && !ctrlMetaKey) {
      setQuery(actualOptions[focusOption], index);
    } else if (enter && !shiftKey) {
      runQuery();
    }
  };

  useEffect(() => {
    if (!wrapperEl.current) return;
    const target = wrapperEl.current.childNodes[focusOption] as HTMLElement;
    if (target?.scrollIntoView) target.scrollIntoView({block: "center"});
  }, [focusOption]);

  return <Box ref={autocompleteAnchorEl}>
    <TextField
      defaultValue={query}
      fullWidth
      label={label}
      multiline
      focused={!!query}
      error={!!error}
      onFocus={() => setFocusField(true)}
      onKeyDown={handleKeyDown}
      onChange={(e) => setQuery(e.target.value, index)}
    />
    <Popper open={openAutocomplete} anchorEl={autocompleteAnchorEl.current} placement="bottom-start" sx={{zIndex: 3}}>
      <ClickAwayListener onClickAway={() => setOpenAutocomplete(false)}>
        <Paper elevation={3} sx={{ maxHeight: 300, overflow: "auto" }}>
          <MenuList ref={wrapperEl} dense>
            {actualOptions.map((item, i) =>
              <MenuItem
                id={`$autocomplete$${item}`}
                key={item}
                sx={{bgcolor: `rgba(0, 0, 0, ${i === focusOption ? 0.12 : 0})`}}
                onClick={() => {
                  setQuery(item, index);
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
