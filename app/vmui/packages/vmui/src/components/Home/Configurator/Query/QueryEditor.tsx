import React, {FC, useEffect, useMemo, useRef, useState} from "preact/compat";
import {KeyboardEvent} from "react";
import {ErrorTypes} from "../../../../types";
import Popper from "@mui/material/Popper";
import TextField from "@mui/material/TextField";
import Box from "@mui/material/Box";
import Paper from "@mui/material/Paper";
import MenuItem from "@mui/material/MenuItem";
import MenuList from "@mui/material/MenuList";

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
}

const QueryEditor: FC<QueryEditorProps> = ({
  index,
  query,
  setHistoryIndex,
  setQuery,
  runQuery,
  autocomplete,
  error,
  queryOptions
}) => {

  const [downMetaKeys, setDownMetaKeys] = useState<string[]>([]);
  const [focusField, setFocusField] = useState(false);
  const [focusOption, setFocusOption] = useState(-1);
  const autocompleteAnchorEl = useRef<HTMLDivElement>(null);
  const wrapperEl = useRef<HTMLUListElement>(null);

  const openAutocomplete = useMemo(() => {
    return !(!autocomplete || downMetaKeys.length || query.length < 2 || !focusField);
  }, [query, downMetaKeys, autocomplete, focusField]);

  const actualOptions = useMemo(() => {
    if (!openAutocomplete) return [];
    try {
      const regexp = new RegExp(String(query), "i");
      return queryOptions.filter((item) => regexp.test(item) && item !== query);
    } catch (e) {
      return [];
    }
  }, [autocomplete, query, queryOptions]);

  const handleKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const {key, ctrlKey, metaKey, shiftKey} = e;
    if (ctrlKey || metaKey) setDownMetaKeys([...downMetaKeys, e.key]);
    if (key === "ArrowUp" && openAutocomplete && actualOptions.length) {
      e.preventDefault();
      setFocusOption((prev) => prev === 0 ? 0 : prev - 1);
    } else if (key === "ArrowDown" && openAutocomplete && actualOptions.length) {
      e.preventDefault();
      setFocusOption((prev) => prev >= actualOptions.length - 1 ? actualOptions.length - 1 : prev + 1);
    } else if (key === "Enter" && openAutocomplete && actualOptions.length && !shiftKey) {
      e.preventDefault();
      setQuery(actualOptions[focusOption], index);
    }
    return true;
  };

  const handleKeyUp = (e: KeyboardEvent<HTMLDivElement>) => {
    const {key, ctrlKey, metaKey} = e;
    if (downMetaKeys.includes(key)) setDownMetaKeys(downMetaKeys.filter(k => k !== key));
    const ctrlMetaKey = ctrlKey || metaKey;
    if (key === "Enter" && ctrlMetaKey) {
      runQuery();
    } else if (key === "ArrowUp" && ctrlMetaKey) {
      setHistoryIndex(-1, index);
    } else if (key === "ArrowDown" && ctrlMetaKey) {
      setHistoryIndex(1, index);
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
      label={`Query ${index + 1}`}
      multiline
      error={!!error}
      onFocus={() => setFocusField(true)}
      onBlur={() => setFocusField(false)}
      onKeyUp={handleKeyUp}
      onKeyDown={handleKeyDown}
      onChange={(e) => setQuery(e.target.value, index)}
    />
    <Popper open={openAutocomplete} anchorEl={autocompleteAnchorEl.current} placement="bottom-start">
      <Paper elevation={3} sx={{ maxHeight: 300, overflow: "auto" }}>
        <MenuList ref={wrapperEl} dense>
          {actualOptions.map((item, i) =>
            <MenuItem key={item} sx={{bgcolor: `rgba(0, 0, 0, ${i === focusOption ? 0.12 : 0})`}}>
              {item}
            </MenuItem>)}
        </MenuList>
      </Paper>
    </Popper>
  </Box>;
};

export default QueryEditor;