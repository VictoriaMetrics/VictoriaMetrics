import React, {FC, useEffect, useState} from "preact/compat";
import {KeyboardEvent} from "react";
import {ErrorTypes} from "../../../../types";
import Autocomplete from "@mui/material/Autocomplete";
import TextField from "@mui/material/TextField";

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

  const [value, setValue] = useState(query);
  const [downMetaKeys, setDownMetaKeys] = useState<string[]>([]);

  useEffect(() => {
    setValue(decodeURIComponent(query));
  }, [query]);

  const handleKeyDown = (e: KeyboardEvent<HTMLDivElement>): void => {
    if (e.ctrlKey || e.metaKey) setDownMetaKeys([...downMetaKeys, e.key]);
  };

  const handleKeyUp = (e: KeyboardEvent<HTMLDivElement>): void => {
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

  return <Autocomplete
    freeSolo
    fullWidth
    disableClearable
    options={autocomplete && !downMetaKeys.length ? queryOptions : []}
    onChange={(event, value) => setQuery(value, index)}
    onKeyDown={handleKeyDown}
    onKeyUp={handleKeyUp}
    value={value}
    renderInput={(params) =>
      <TextField
        {...params}
        label={`Query ${index + 1}`}
        multiline
        error={!!error}
        onChange={(e) => setQuery(e.target.value, index)}
      />
    }
  />;
};

export default QueryEditor;