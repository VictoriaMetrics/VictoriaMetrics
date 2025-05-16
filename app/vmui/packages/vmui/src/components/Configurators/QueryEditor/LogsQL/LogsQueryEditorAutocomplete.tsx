import React, { FC, useCallback, useEffect, useMemo, useState } from "preact/compat";
import Autocomplete, { AutocompleteOptions } from "../../../Main/Autocomplete/Autocomplete";
import { AUTOCOMPLETE_LIMITS } from "../../../../constants/queryAutocomplete";
import { QueryEditorAutocompleteProps } from "../QueryEditor";
import { getContextData, splitLogicalParts } from "./parser";
import { ContextType, LogicalPart, LogicalPartType } from "./types";
import { useFetchLogsQLOptions } from "./useFetchLogsQLOptions";
import { pipeList } from "./pipes";

const LogsQueryEditorAutocomplete: FC<QueryEditorAutocompleteProps> = ({
  value,
  anchorEl,
  caretPosition,
  hasHelperText,
  onSelect,
  onFoundOptions
}) => {
  const [offsetPos, setOffsetPos] = useState({ top: 0, left: 0 });

  const fullValue = useMemo(() => {
    if (caretPosition[0] !== caretPosition[1]) return { valueBeforeCursor: value, valueAfterCursor: "" };
    const valueBeforeCursor = value.substring(0, caretPosition[0]);
    const valueAfterCursor = value.substring(caretPosition[1]);
    return { valueBeforeCursor, valueAfterCursor };
  }, [value, caretPosition]);

  const logicalParts = useMemo(() => {
    return splitLogicalParts(value);
  }, [value]);

  const contextData = useMemo(() => {
    if (caretPosition[0] !== caretPosition[1]) return;
    const part = logicalParts.find(p => caretPosition[0] >= p.position[0] && caretPosition[0] <= p.position[1]);
    if (!part) return;
    const cursorStartPosition = caretPosition[0] - part.position[0];
    const prevPart = logicalParts.find(p => p.id === part.id - 1);
    const queryBeforeIncompleteFilter = prevPart ? value.substring(0, prevPart.position[1] + 1) : undefined;
    return {
      ...part,
      queryBeforeIncompleteFilter,
      query: value,
      ...getContextData(part, cursorStartPosition)
    };
  }, [logicalParts, caretPosition]);

  const { fieldNames, fieldValues, loading } = useFetchLogsQLOptions(contextData);

  const options = useMemo(() => {
    switch (contextData?.contextType) {
      case ContextType.FilterName:
      case ContextType.FilterUnknown:
        return fieldNames;
      case ContextType.FilterValue:
        return fieldValues;
      case ContextType.PipeName:
        return pipeList;
      case ContextType.FilterOrPipeName:
        return [...fieldNames, ...pipeList];
      default:
        return [];
    }
  }, [contextData, fieldNames, fieldValues]);

  const getUpdatedValue = (insertValue: string, logicalParts: LogicalPart[], id?: number) => {
    return logicalParts.reduce((acc, part) => {
      const value = part.id === id ? insertValue : part.value;
      const separator = part.separator === "|" ? " | " : " ";
      return `${acc}${separator}${value}`;
    }, "").trim();
  };

  const getModifyInsert = (insert: string, contextType: ContextType, value = "", insertType?: string) => {
    let modifiedInsert = insert;

    if (insertType === ContextType.FilterName) {
      modifiedInsert += ":";
    } else if (contextType === ContextType.FilterValue) {
      const insertWithQuotes = value.startsWith("_stream:") ? modifiedInsert : `${JSON.stringify(modifiedInsert)}`;
      modifiedInsert = `${contextData?.filterName || ""}${contextData?.operator || ":"}${insertWithQuotes}`;
    }

    return modifiedInsert;
  };

  const handleSelect = useCallback((insert: string, item: AutocompleteOptions) => {
    const {
      id,
      contextType = ContextType.FilterUnknown,
      value = "",
      position = [0, 0]
    } = contextData || {};

    const insertValue = getModifyInsert(insert, contextType, value, item.type);
    const newValue = getUpdatedValue(insertValue, logicalParts, id);
    const logicalPart = logicalParts.find(p => p.id === id);
    const getPositionCorrection = () => {
      if (logicalPart?.type === LogicalPartType.FilterOrPipe) return 1;
      if (item.type === ContextType.PipeName) return 1;
      return 0;
    };
    const updatedPosition = (position[0] || 1) + insertValue.length + getPositionCorrection();

    onSelect(newValue, updatedPosition);
  }, [contextData, logicalParts]);


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
    span.appendChild(document.createTextNode(fullValue.valueBeforeCursor || ""));
    span.appendChild(marker);
    span.appendChild(document.createTextNode(fullValue.valueAfterCursor || ""));
    document.body.appendChild(span);

    const spanRect = span.getBoundingClientRect();
    const markerRect = marker.getBoundingClientRect();

    const leftOffset = markerRect.left - spanRect.left;
    const topOffset = markerRect.bottom - spanRect.bottom - (hasHelperText ? lineHeight : 0);
    setOffsetPos({ top: topOffset, left: leftOffset });

    span.remove();
    marker.remove();
  }, [anchorEl, caretPosition, hasHelperText, fullValue]);

  return (
    <>
      <Autocomplete
        loading={loading}
        disabledFullScreen
        value={contextData?.valueContext || ""}
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

export default LogsQueryEditorAutocomplete;
