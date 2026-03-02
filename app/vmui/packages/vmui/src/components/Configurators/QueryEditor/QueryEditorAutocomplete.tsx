import { FC, useState, useEffect, useMemo, useCallback } from "preact/compat";
import Autocomplete from "../../Main/Autocomplete/Autocomplete";
import { useFetchQueryOptions } from "../../../hooks/useFetchQueryOptions";
import useGetMetricsQL from "../../../hooks/useGetMetricsQL";
import { QueryContextType } from "../../../types";
import { AUTOCOMPLETE_LIMITS } from "../../../constants/queryAutocomplete";
import { QueryEditorAutocompleteProps } from "./QueryEditor";
import { getExprLastPart, getValueByContext, getContext } from "./autocompleteUtils";
import { extractCurrentLabel, extractLabelMatchers, extractMetric, splitByCursor } from "./utils/parser";
import { escapeLabelName } from "../../../utils/metric";

const QueryEditorAutocomplete: FC<QueryEditorAutocompleteProps> = ({
  value,
  anchorEl,
  caretPosition,
  hasHelperText,
  includeFunctions,
  onSelect,
  onFoundOptions
}) => {
  const [offsetPos, setOffsetPos] = useState({ top: 0, left: 0 });
  const metricsqlFunctions = useGetMetricsQL(includeFunctions);

  const values = useMemo(() => {
    return splitByCursor(value, caretPosition);
  }, [value, caretPosition]);

  const exprLastPart = useMemo(() => {
    return getExprLastPart(values.beforeCursor);
  }, [values.beforeCursor]);

  const metric = useMemo(() => {
    return extractMetric(exprLastPart);
  }, [exprLastPart]);

  const label = useMemo(() => {
    return extractCurrentLabel(exprLastPart);
  }, [exprLastPart]);

  const context = useMemo(() => {
    return getContext(values.beforeCursor, metric, label);
  }, [values.beforeCursor, metric, label]);

  const valueByContext = useMemo(() => {
    return getValueByContext(values.beforeCursor);
  }, [values.beforeCursor]);

  const labelMatchers = useMemo(() => {
    return extractLabelMatchers(values.beforeCursor, label);
  }, [values.beforeCursor, label]);

  const { metrics, labels, labelValues, loading } = useFetchQueryOptions({
    valueByContext,
    metric,
    label,
    context,
    labelMatchers,
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
  }, [context, metrics, labels, labelValues, metricsqlFunctions]);

  const handleSelect = useCallback((insert: string) => {
    // Find the start and end of valueByContext in the query string
    const beforeCursor = values.beforeCursor;
    let valueAfterCursor = values.afterCursor;
    const startIndexOfValueByContext = beforeCursor.lastIndexOf(valueByContext, caretPosition[0]);
    const endIndexOfValueByContext = startIndexOfValueByContext + valueByContext.length;

    // Split the original string into parts: before, during, and after valueByContext
    const beforeValueByContext = beforeCursor.substring(0, startIndexOfValueByContext);
    const afterValueByContext = beforeCursor.substring(endIndexOfValueByContext);

    // Add quotes around the value if the context is labelValue
    if (context === QueryContextType.labelValue) {
      const quote = "\"";
      valueAfterCursor = valueAfterCursor.replace(/^[^\s"|},]*/, "");
      const needsOpenQuote = /(?:=|!=|=~|!~)$/.test(beforeValueByContext);
      const needsCloseQuote = valueAfterCursor.trim()[0] !== "\"";
      insert = `${needsOpenQuote ? quote : ""}${insert}${needsCloseQuote ? quote : ""}`;
    }

    if (context === QueryContextType.label) {
      insert = escapeLabelName(insert);
      valueAfterCursor = valueAfterCursor.replace(/^[^\s=!,{}()"|+\-/*^]*/, "");
    }

    if (context === QueryContextType.metricsql) {
      valueAfterCursor = valueAfterCursor.replace(/^[^\s[\]{}()"|+\-/*^]*/, "");
    }
    // Assemble the new value with the inserted text
    const newVal = `${beforeValueByContext}${insert}${afterValueByContext}${valueAfterCursor}`;
    onSelect(newVal, beforeValueByContext.length + insert.length);
  }, [values.beforeCursor, values.afterCursor, valueByContext, caretPosition, context, onSelect]);

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
  }, [anchorEl, caretPosition, hasHelperText, values.beforeCursor, values.afterCursor]);

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
