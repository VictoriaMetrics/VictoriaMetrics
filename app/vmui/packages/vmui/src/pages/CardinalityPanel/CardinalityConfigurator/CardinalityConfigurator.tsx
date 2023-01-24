import React, { FC } from "react";
import QueryEditor from "../../../components/Configurators/QueryEditor/QueryEditor";
import { useFetchQueryOptions } from "../../../hooks/useFetchQueryOptions";
import { ErrorTypes } from "../../../types";
import { useQueryDispatch, useQueryState } from "../../../state/query/QueryStateContext";
import Switch from "../../../components/Main/Switch/Switch";
import { PlayIcon, QuestionIcon } from "../../../components/Main/Icons";
import Button from "../../../components/Main/Button/Button";
import TextField from "../../../components/Main/TextField/TextField";
import "./style.scss";
import { useMemo } from "preact/compat";
import Tooltip from "../../../components/Main/Tooltip/Tooltip";

export interface CardinalityConfiguratorProps {
  onSetHistory: (step: number) => void;
  onSetQuery: (query: string) => void;
  onRunQuery: () => void;
  onTopNChange: (value: string) => void;
  onFocusLabelChange: (value: string) => void;
  query: string;
  topN: number;
  error?: ErrorTypes | string;
  totalSeries: number;
  totalLabelValuePairs: number;
  date: string | null;
  match: string | null;
  focusLabel: string | null;
}

const CardinalityConfigurator: FC<CardinalityConfiguratorProps> = ({
  topN,
  error,
  query,
  onSetHistory,
  onRunQuery,
  onSetQuery,
  onTopNChange,
  onFocusLabelChange,
  totalSeries,
  totalLabelValuePairs,
  date,
  match,
  focusLabel
}) => {
  const { autocomplete } = useQueryState();
  const queryDispatch = useQueryDispatch();

  const { queryOptions } = useFetchQueryOptions();

  const errorTopN = useMemo(() => topN < 1 ? "Number must be bigger than zero" : "", [topN]);

  const onChangeAutocomplete = () => {
    queryDispatch({ type: "TOGGLE_AUTOCOMPLETE" });
  };

  const handleArrowUp = () => {
    onSetHistory(-1);
  };

  const handleArrowDown = () => {
    onSetHistory(1);
  };

  return <div className="vm-cardinality-configurator vm-block">
    <div className="vm-cardinality-configurator-controls">
      <div className="vm-cardinality-configurator-controls__query">
        <QueryEditor
          value={query || match || ""}
          autocomplete={autocomplete}
          options={queryOptions}
          error={error}
          onArrowUp={handleArrowUp}
          onArrowDown={handleArrowDown}
          onEnter={onRunQuery}
          onChange={onSetQuery}
          label={"Time series selector"}
        />
      </div>
      <div className="vm-cardinality-configurator-controls__item">
        <TextField
          label="Number of entries per table"
          type="number"
          value={topN}
          error={errorTopN}
          onChange={onTopNChange}
        />
      </div>
      <div className="vm-cardinality-configurator-controls__item">
        <TextField
          label="Focus label"
          type="text"
          value={focusLabel || ""}
          onChange={onFocusLabelChange}
        />
      </div>
    </div>
    <div className="vm-cardinality-configurator-bottom__autocomplete">
      <Switch
        label={"Autocomplete"}
        value={autocomplete}
        onChange={onChangeAutocomplete}
      />
    </div>
    <div className="vm-cardinality-configurator-bottom">
      <div className="vm-cardinality-configurator-bottom__info">
        Analyzed <b>{totalSeries}</b> series with <b>{totalLabelValuePairs}</b> &quot;label=value&quot; pairs
        at <b>{date}</b>{match && <span> for series selector <b>{match}</b></span>}.
        Show top {topN} entries per table.
      </div>
      <a
        className="vm-cardinality-configurator-bottom__docs"
        href="https://victoriametrics.com/blog/cardinality-explorer/"
        target="_blank"
        rel="help noreferrer"
      >
        <Tooltip title="Example of using">
          <Button
            variant="text"
            startIcon={<QuestionIcon/>}
          />
        </Tooltip>
      </a>
      <Button
        startIcon={<PlayIcon/>}
        onClick={onRunQuery}
      >
        Execute Query
      </Button>
    </div>
  </div>;
};

export default CardinalityConfigurator;
