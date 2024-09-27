import React, { FC, useState, useEffect, useMemo, useCallback } from "preact/compat";
import Autocomplete, { AutocompleteOptions } from "../../Main/Autocomplete/Autocomplete";
import { useFetchQueryOptions } from "../../../hooks/useFetchQueryOptions";
import { escapeRegexp, hasUnclosedQuotes } from "../../../utils/regexp";
import useGetMetricsQL from "../../../hooks/useGetMetricsQL";
import { QueryContextType } from "../../../types";
import { AUTOCOMPLETE_LIMITS } from "../../../constants/queryAutocomplete";

interface QueryEditorAutocompleteProps {
  value: string;
  anchorEl: React.RefObject<HTMLElement>;
  caretPosition: [number, number]; // [start, end]
  hasHelperText: boolean;
  onSelect: (val: string, caretPosition: number) => void;
  onFoundOptions: (val: AutocompleteOptions[]) => void;
}

const QueryEditorAutocomplete: FC<QueryEditorAutocompleteProps> = ({
  value,
  anchorEl,
  caretPosition,
  hasHelperText,
  onSelect,
  onFoundOptions
}) => {
  const [offsetPos, setOffsetPos] = useState({ top: 0, left: 0 });
  const metricsqlFunctions = useGetMetricsQL();

  const values = useMemo(() => {
    if (caretPosition[0] !== caretPosition[1]) return { beforeCursor: value, afterCursor: "" };
    const beforeCursor = value.substring(0, caretPosition[0]);
    const afterCursor = value.substring(caretPosition[1]);
    return { beforeCursor, afterCursor };
  }, [value, caretPosition]);

  const exprLastPart = useMemo(() => {
    const regexpSplit = /\s(or|and|unless|default|ifnot|if|group_left|group_right)\s|}|\+|\|-|\*|\/|\^/i;
    const parts = values.beforeCursor.split(regexpSplit);
    return parts[parts.length - 1];
  }, [values]);

  const metric = useMemo(() => {
    const regex1 = /\w+\((?<metricName>[^)]+)\)\s+(by|without|on|ignoring)\s*\(\w*/gi;
    const matchAlt = [...exprLastPart.matchAll(regex1)];
    if (matchAlt.length > 0 && matchAlt[0].groups && matchAlt[0].groups.metricName) {
      return matchAlt[0].groups.metricName;
    }

    const regex2 = /^\s*\b(?<metricName>[^{}(),\s]+)(?={|$)/g;
    const match = [...exprLastPart.matchAll(regex2)];
    if (match.length > 0 && match[0].groups && match[0].groups.metricName) {
      return match[0].groups.metricName;
    }

    return "";
  }, [exprLastPart]);

  const label = useMemo(() => {
    const regexp = /[a-z_:-][\w\-.:/]*\b(?=\s*(=|!=|=~|!~))/g;
    const match = exprLastPart.match(regexp);
    return match ? match[match.length - 1] : "";
  }, [exprLastPart]);

  const shouldSuppressAutoSuggestion = (value: string) => {
    const pattern = /([{(),+\-*/^]|\b(?:or|and|unless|default|ifnot|if|group_left|group_right|by|without|on|ignoring)\b)/i;
    const parts = value.split(/\s+/);
    const partsCount = parts.length;
    const lastPart = parts[partsCount - 1];
    const preLastPart = parts[partsCount - 2];

    const hasEmptyPartAndQuotes = !lastPart && hasUnclosedQuotes(value);
    const suppressPreLast = (!lastPart || parts.length > 1) && !pattern.test(preLastPart);
    return hasEmptyPartAndQuotes || suppressPreLast;
  };

  const context = useMemo(() => {
    const valueBeforeCursor = values.beforeCursor.trim();
    const endOfClosedBrackets = ["}", ")"].some(char => valueBeforeCursor.endsWith(char));
    const endOfClosedQuotes = !hasUnclosedQuotes(valueBeforeCursor) && ["`", "'", "\""].some(char => valueBeforeCursor.endsWith(char));
    if (!values.beforeCursor || endOfClosedBrackets || endOfClosedQuotes || shouldSuppressAutoSuggestion(values.beforeCursor)) {
      return QueryContextType.empty;
    }

    const labelRegexp = /(?:by|without|on|ignoring)\s*\(\s*[^)]*$|\{[^}]*$/i;
    const patternLabelValue = `(${escapeRegexp(metric)})?{?.+${escapeRegexp(label)}(=|!=|=~|!~)"?([^"]*)$`;
    const labelValueRegexp = new RegExp(patternLabelValue, "g");

    switch (true) {
      case labelValueRegexp.test(values.beforeCursor):
        return QueryContextType.labelValue;
      case labelRegexp.test(values.beforeCursor):
        return QueryContextType.label;
      default:
        return QueryContextType.metricsql;
    }
  }, [values, metric, label]);

  const valueByContext = useMemo(() => {
    const wordMatch = values.beforeCursor.match(/([\w_.:]+(?![},]))$/);
    return wordMatch ? wordMatch[0] : "";
  }, [values.beforeCursor]);

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

  const handleSelect = useCallback((insert: string) => {
    // Find the start and end of valueByContext in the query string
    const value = values.beforeCursor;
    let valueAfterCursor = values.afterCursor;
    const startIndexOfValueByContext = value.lastIndexOf(valueByContext, caretPosition[0]);
    const endIndexOfValueByContext = startIndexOfValueByContext + valueByContext.length;

    // Split the original string into parts: before, during, and after valueByContext
    const beforeValueByContext = value.substring(0, startIndexOfValueByContext);
    const afterValueByContext = value.substring(endIndexOfValueByContext);

    // Add quotes around the value if the context is labelValue
    if (context === QueryContextType.labelValue) {
      const quote = "\"";
      valueAfterCursor = valueAfterCursor.replace(/^[^\s"|},]*/, "");
      const needsOpenQuote = /(?:=|!=|=~|!~)$/.test(beforeValueByContext);
      const needsCloseQuote = valueAfterCursor.trim()[0] !== "\"";
      insert = `${needsOpenQuote ? quote : ""}${insert}${needsCloseQuote ? quote : ""}`;
    }

    if (context === QueryContextType.label) {
      valueAfterCursor = valueAfterCursor.replace(/^[^\s=!,{}()"|+\-/*^]*/, "");
    }

    if (context === QueryContextType.metricsql) {
      valueAfterCursor = valueAfterCursor.replace(/^[^\s[\]{}()"|+\-/*^]*/, "");
    }
    // Assemble the new value with the inserted text
    const newVal = `${beforeValueByContext}${insert}${afterValueByContext}${valueAfterCursor}`;
    onSelect(newVal, beforeValueByContext.length + insert.length);
  }, [values]);

  useEffect(() => {
    if (!anchorEl.current) {
      setOffsetPos({ top: 0, left: 0 });
      return;
    }

    const element = anchorEl.current.querySelector("textarea") || anchorEl.current;
    const style = window.getComputedStyle(element);
    const fontSize = `${style.getPropertyValue("font-size")}`;
    const fontFamily = `${style.getPropertyValue("font-family")}`;
    const lineHeight = parseInt(`${style.getPropertyValue("line-height")}`);

    const span = document.createElement("div");
    span.style.font = `${fontSize} ${fontFamily}`;
    span.style.padding = style.getPropertyValue("padding");
    span.style.lineHeight = `${lineHeight}px`;
    span.style.width = `${element.offsetWidth}px`;
    span.style.maxWidth = `${element.offsetWidth}px`;
    span.style.whiteSpace = style.getPropertyValue("white-space");
    span.style.overflowWrap = style.getPropertyValue("overflow-wrap");

    const marker = document.createElement("span");
    span.appendChild(document.createTextNode(values.beforeCursor));
    span.appendChild(marker);
    span.appendChild(document.createTextNode(values.afterCursor));
    document.body.appendChild(span);

    const spanRect = span.getBoundingClientRect();
    const markerRect = marker.getBoundingClientRect();

    const leftOffset = markerRect.left - spanRect.left;
    const topOffset = markerRect.bottom - spanRect.bottom - (hasHelperText ? lineHeight : 0);
    setOffsetPos({ top: topOffset, left: leftOffset });

    span.remove();
    marker.remove();
  }, [anchorEl, caretPosition, hasHelperText]);

  return (
    <>
      <Autocomplete
        loading={loading}
        disabledFullScreen
        value={valueByContext}
        options={options}
        anchor={anchorEl}
        minLength={0}
        offset={offsetPos}
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
