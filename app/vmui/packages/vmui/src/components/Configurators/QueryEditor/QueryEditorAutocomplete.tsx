import React, { FC, Ref, useState, useEffect, useMemo } from "preact/compat";
import Autocomplete, { AutocompleteOptions } from "../../Main/Autocomplete/Autocomplete";
import { useFetchQueryOptions } from "../../../hooks/useFetchQueryOptions";
import { getTextWidth } from "../../../utils/uplot";
import { escapeRegExp } from "../../../utils/regexp";
import useGetMetricsQL from "../../../hooks/useGetMetricsQL";

enum ContextType {
  empty = "empty",
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
  const [leftOffset, setLeftOffset] = useState(0);
  const metricsqlFunctions = useGetMetricsQL();

  const metric = useMemo(() => {
    const regexp = /\b[^{}(),\s]+(?={|$)/g;
    const match = value.match(regexp);
    return match ? match[0] : "";
  }, [value]);

  const label = useMemo(() => {
    const regexp = /[a-z_]\w*(?=\s*(=|!=|=~|!~))/g;
    const match = value.match(regexp);
    return match ? match[match.length - 1] : "";
  }, [value]);


  const metricRegexp = new RegExp(`\\(?(${escapeRegExp(metric)})$`, "g");
  const labelRegexp = /[{.,].?(\w+)$/gm;
  const valueRegexp = new RegExp(`(${escapeRegExp(metric)})?{?.+${escapeRegExp(label)}="?([^"]*)$`, "g");

  const context = useMemo(() => {
    [metricRegexp, labelRegexp, valueRegexp].forEach(regexp => regexp.lastIndex = 0);
    switch (true) {
      case valueRegexp.test(value):
        return ContextType.value;
      case labelRegexp.test(value):
        return ContextType.label;
      case metricRegexp.test(value):
        return ContextType.metricsql;
      default:
        return ContextType.empty;
    }
  }, [value, valueRegexp, labelRegexp, metricRegexp]);

  const { metrics, labels, values } = useFetchQueryOptions({ metric, label });

  const options = useMemo(() => {
    switch (context) {
      case ContextType.metricsql:
        return [...metrics, ...metricsqlFunctions];
      case ContextType.label:
        return labels;
      case ContextType.value:
        return values;
      default:
        return [];
    }
  }, [context, metrics, labels, values]);

  const valueByContext = useMemo(() => {
    if (value.length !== caretPosition[1]) return value;

    const wordMatch = value.match(/([\w_]+)$/) || [];
    return wordMatch[1] || "";
  }, [context, caretPosition, value]);

  const handleSelect = (insert: string) => {
    const wordMatch = value.match(/([\w_]+)$/);
    const wordMatchIndex = wordMatch?.index !== undefined ? wordMatch.index : value.length;
    const beforeInsert = value.substring(0, wordMatchIndex);
    const afterInsert = value.substring(wordMatchIndex + (wordMatch?.[1].length || 0));

    if (context === ContextType.value) {
      const quote = "\"";
      const needsQuote = beforeInsert[beforeInsert.length - 1] !== quote;
      insert = `${needsQuote ? quote : ""}${insert}${quote}`;
    }

    const newVal = `${beforeInsert}${insert}${afterInsert}`;
    onSelect(newVal);
  };

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

  return (
    <Autocomplete
      disabledFullScreen
      value={valueByContext}
      options={options}
      anchor={anchorEl}
      minLength={context === ContextType.metricsql ? 2 : 0}
      offset={{ top: 0, left: leftOffset }}
      onSelect={handleSelect}
      onFoundOptions={onFoundOptions}
    />
  );
};

export default QueryEditorAutocomplete;
