import React, { FC, Ref, useState, useEffect, useMemo } from "preact/compat";
import Autocomplete, { AutocompleteOptions } from "../../Main/Autocomplete/Autocomplete";
import { useFetchQueryOptions } from "../../../hooks/useFetchQueryOptions";
import { getTextWidth } from "../../../utils/uplot";
import { escapeRegexp } from "../../../utils/regexp";
import useGetMetricsQL from "../../../hooks/useGetMetricsQL";
import { QueryContextType } from "../../../types";
import { AUTOCOMPLETE_LIMITS } from "../../../constants/queryAutocomplete";

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

  const exprLastPart = useMemo(() => {
    const parts = value.split("}");
    return parts[parts.length - 1];
  }, [value]);

  const metric = useMemo(() => {
    const regexp = /\b[^{}(),\s]+(?={|$)/g;
    const match = exprLastPart.match(regexp);
    return match ? match[0] : "";
  }, [exprLastPart]);

  const label = useMemo(() => {
    const regexp = /[a-z_:-][\w\-.:/]*\b(?=\s*(=|!=|=~|!~))/g;
    const match = exprLastPart.match(regexp);
    return match ? match[match.length - 1] : "";
  }, [exprLastPart]);

  const context = useMemo(() => {
    if (!value || value.endsWith("}")) return QueryContextType.empty;

    const labelRegexp = /\{[^}]*?(\w+)*$/gm;
    const labelValueRegexp = new RegExp(`(${escapeRegexp(metric)})?{?.+${escapeRegexp(label)}(=|!=|=~|!~)"?([^"]*)$`, "g");

    switch (true) {
      case labelValueRegexp.test(value):
        return QueryContextType.labelValue;
      case labelRegexp.test(value):
        return QueryContextType.label;
      default:
        return QueryContextType.metricsql;
    }
  }, [value, metric, label]);

  const valueByContext = useMemo(() => {
    const wordMatch = value.match(/([\w_\-.:/]+(?![},]))$/);
    return wordMatch ? wordMatch[0] : "";
  }, [value]);

  const { metrics, labels, labelValues, loading } = useFetchQueryOptions({
    valueByContext,
    metric,
    label,
    context,
  });

  const options = useMemo(() => {
    switch (context) {
      case QueryContextType.metricsql:
        return [...metrics, ...metricsqlFunctions];
      case QueryContextType.label:
        return labels;
      case QueryContextType.labelValue:
        return labelValues;
      default:
        return [];
    }
  }, [context, metrics, labels, labelValues]);

  const handleSelect = (insert: string) => {
    // Find the start and end of valueByContext in the query string
    const startIndexOfValueByContext = value.lastIndexOf(valueByContext, caretPosition[0]);
    const endIndexOfValueByContext = startIndexOfValueByContext + valueByContext.length;

    // Split the original string into parts: before, during, and after valueByContext
    const beforeValueByContext = value.substring(0, startIndexOfValueByContext);
    const afterValueByContext = value.substring(endIndexOfValueByContext);

    // Add quotes around the value if the context is labelValue
    if (context === QueryContextType.labelValue) {
      const quote = "\"";
      const needsQuote = /(?:=|!=|=~|!~)$/.test(beforeValueByContext);
      insert = `${needsQuote ? quote : ""}${insert}`;
    }

    // Assemble the new value with the inserted text
    const newVal = `${beforeValueByContext}${insert}${afterValueByContext}`;
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
    <>
      <Autocomplete
        loading={loading}
        disabledFullScreen
        value={valueByContext}
        options={options}
        anchor={anchorEl}
        minLength={0}
        offset={{ top: 0, left: leftOffset }}
        onSelect={handleSelect}
        onFoundOptions={onFoundOptions}
        maxDisplayResults={{
          limit: AUTOCOMPLETE_LIMITS.displayResults,
          message: "Please, specify the query more precisely."
        }}
      />
    </>
  );
};

export default QueryEditorAutocomplete;
