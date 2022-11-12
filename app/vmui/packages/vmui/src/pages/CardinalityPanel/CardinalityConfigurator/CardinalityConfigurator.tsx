import React, { ChangeEvent, FC } from "react";
import QueryEditor from "../../../components/Configurators/QueryEditor/QueryEditor";
import { useFetchQueryOptions } from "../../../hooks/useFetchQueryOptions";
import { ErrorTypes } from "../../../types";
import { useQueryDispatch, useQueryState } from "../../../state/query/QueryStateContext";
import Switch from "../../../components/Main/Switch/Switch";
import { PlayCircleOutlineIcon, PlayIcon } from "../../../components/Main/Icons";
import Button from "../../../components/Main/Button/Button";
import TextField from "../../../components/Main/TextField/TextField";
import "./style.scss";

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

  const onChangeAutocomplete = () => {
    queryDispatch({ type: "TOGGLE_AUTOCOMPLETE" });
  };

  return <div className="vm-cardinality-configurator vm-block">
    <div className="vm-cardinality-configurator-controls">
      <div className="vm-cardinality-configurator-controls__query">
        <QueryEditor
          value={query || match || ""}
          autocomplete={autocomplete}
          options={queryOptions}
          error={error}
          onArrowUp={() => onSetHistory(-1)}
          onArrowDown={() => onSetHistory(1)}
          onEnter={onRunQuery}
          onChange={(value) => onSetQuery(value)}
          label={"Time series selector"}
        />
      </div>
      <div className="vm-cardinality-configurator-controls__item">
        <TextField
          label="Number of entries per table"
          type="number"
          value={topN}
          error={topN < 1 ? "Number must be bigger than zero" : ""}
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
      <div className="vm-cardinality-configurator-controls__item">
        <Switch
          label={"Autocomplete"}
          value={autocomplete}
          onChange={onChangeAutocomplete}
        />
      </div>
    </div>
    <div className="vm-cardinality-configurator-bottom">
      <div className="vm-cardinality-configurator-bottom__info">
        Analyzed <b>{totalSeries}</b> series with <b>{totalLabelValuePairs}</b> &quot;label=value&quot; pairs
        at <b>{date}</b>{match && <span> for series selector <b>{match}</b></span>}.
        Show top {topN} entries per table.
      </div>
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
