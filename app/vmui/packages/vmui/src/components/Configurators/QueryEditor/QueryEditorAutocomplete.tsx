import React, { FC, Ref, useState, useEffect, useMemo } from "preact/compat";
import Autocomplete, { AutocompleteOptions } from "../../Main/Autocomplete/Autocomplete";
import { useFetchQueryOptions } from "../../../hooks/useFetchQueryOptions";
import { getTextWidth } from "../../../utils/uplot";
import metricsqlFunctions from "../../../constants/metricsqlFunctions";

enum CONTEXT_SYNTAX {
  metricsql = "metricsql",
  label = "label",
  value = "value",
}

interface QueryEditorAutocompleteProps {
  value: string;
  anchorEl: Ref<HTMLInputElement>;
  caretPosition: number[];
  onSelect: (val: string) => void;
  onFoundOptions: (val: AutocompleteOptions[]) => void;
}

const QueryEditorAutocomplete: FC<QueryEditorAutocompleteProps> = ({
  value,
  anchorEl,
  caretPosition,
  onSelect,
  onFoundOptions
}) => {
  const [metric, setMetric] = useState("");
  const [label, setLabel] = useState("");
  const [context, setContext] = useState<CONTEXT_SYNTAX>(CONTEXT_SYNTAX.metricsql);
  const [leftOffset, setLeftOffset] = useState(0);

  const { metricNames, labels, values } = useFetchQueryOptions({ metric, label });

  const options = useMemo(() => {
    if (context === CONTEXT_SYNTAX.label) {
      return labels.map(l => ({ value: l }));
    }
    if (context === CONTEXT_SYNTAX.value) {
      return values.map(l => ({ value: l }));
    }
    return [...metricNames.map(n => ({ value: n })), ...metricsqlFunctions];
  }, [context, metricNames, labels, values]);

  const valueByContext = useMemo(() => {
    const isLabel = context === CONTEXT_SYNTAX.label;
    const isValue = context === CONTEXT_SYNTAX.value;
    if ((isLabel || isValue) && value.length === caretPosition[1]) {
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
    const name = value.replace(/\{.+/, "");
    setMetric(metricNames.includes(name) ? name : "");
  }, [value, metricNames]);

  useEffect(() => {
    if (!metric) {
      setLabel("");
      return;
    }
    const regex = /(?<=\{|\s*,\s*)(?<label>[a-z0-9_]\w*)\s*(?=[=~]?[\s",])/g;
    const matches = Array.from(value.matchAll(regex));
    const lastMatch = matches[matches.length - 1];
    const name = lastMatch?.groups?.label || "";
    setLabel(labels.includes(name) ? name : "");
  }, [value, metric, labels]);

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
    const regexpValue = /(?<=")\s*([^"\s,]*?)\s*(?="|$|,)/;
    if (value.match(regexpLabel)) {
      setContext(CONTEXT_SYNTAX.label);
    } else if (value.match(regexpValue)) {
      setContext(CONTEXT_SYNTAX.value);
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
