import React, { FC, Ref, useState, useEffect, useMemo } from "preact/compat";
import Autocomplete from "../../Main/Autocomplete/Autocomplete";
import { useFetchQueryOptions } from "../../../hooks/useFetchQueryOptions";
import { getTextWidth } from "../../../utils/uplot";

enum CONTEXT_SYNTAX {
  metricsql = "metricsql", // for all syntax
  label = "label", // for label syntax
}

interface QueryEditorAutocompleteProps {
  value: string;
  anchorEl: Ref<HTMLInputElement>;
  caretPosition: number[];
  onSelect: (val: string) => void;
  onFoundOptions: (val: string[]) => void;
}

const QueryEditorAutocomplete: FC<QueryEditorAutocompleteProps> = ({
  value,
  anchorEl,
  caretPosition,
  onSelect,
  onFoundOptions
}) => {
  const [metric, setMetric] = useState("");
  const [context, setContext] = useState<CONTEXT_SYNTAX>(CONTEXT_SYNTAX.metricsql);
  const [leftOffset, setLeftOffset] = useState(0);

  const { metricNames, labels } = useFetchQueryOptions({ metric });

  const options = useMemo(() => {
    if (context === CONTEXT_SYNTAX.label) {
      return labels;
    }
    return metricNames;
  }, [context, metricNames, labels]);

  const valueByContext = useMemo(() => {
    if (context === CONTEXT_SYNTAX.label && value.length === caretPosition[1]) {
      const beforeCaret = value.substring(0, caretPosition[0]);
      const wordMatch = beforeCaret.match(/([\w_]+)$/) || [];
      return wordMatch[1] || "";
    }
    return value;
  }, [context, caretPosition]);

  const handleSelect = (val: string) => {
    const [startCaret] = caretPosition;
    const beforeCaret = value.substring(0, startCaret);
    const wordMatch = beforeCaret.match(/([\w_]+)$/) || [];
    if (wordMatch?.index !== undefined) {
      const newVal = value.substring(0, wordMatch.index) + val + value.substring(wordMatch.index + wordMatch[1].length);
      onSelect(newVal);
    }
  };

  useEffect(() => {
    const name = value.replace(/\{/, "");
    setMetric(metricNames.includes(name) ? name : "");
  }, [value, metricNames]);

  useEffect(() => {
    if (!anchorEl.current) {
      setLeftOffset(0);
      return;
    }

    const style = window.getComputedStyle(anchorEl.current);
    const fontSize = `${style.getPropertyValue("font-size")}`;
    const fontFamily = `${style.getPropertyValue("font-family")}`;
    const offset = getTextWidth(value, `${fontSize} ${fontFamily}`);
    setLeftOffset(offset);
  }, [anchorEl, caretPosition]);

  useEffect(() => {
    const regexpLabel = /(?<={\s*|,\s*)\s*([^\s,=]+?)\s*(?=$|,|})/;
    if (value.match(regexpLabel)) {
      setContext(CONTEXT_SYNTAX.label);
    } else {
      setContext(CONTEXT_SYNTAX.metricsql);
    }
  }, [value]);

  return (
    <Autocomplete
      disabledFullScreen
      value={valueByContext}
      options={options}
      anchor={anchorEl}
      minLength={context === CONTEXT_SYNTAX.label ? 0 : 2}
      offset={{ top: 0, left: leftOffset }}
      onSelect={handleSelect}
      onFoundOptions={onFoundOptions}
    />
  );
};

export default QueryEditorAutocomplete;
