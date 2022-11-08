import React, { FC, useState, useEffect } from "preact/compat";
import QueryEditor from "../../components/Configurators/QueryEditor/QueryEditor";
import AdditionalSettings from "../../components/Configurators/AdditionalSettings/AdditionalSettings";
import { ErrorTypes } from "../../types";
import usePrevious from "../../hooks/usePrevious";
import { MAX_QUERY_FIELDS } from "../../config";
import { useQueryDispatch, useQueryState } from "../../state/query/QueryStateContext";
import { useTimeDispatch } from "../../state/time/TimeStateContext";
import { DeleteIcon, PlayIcon, PlusIcon } from "../../components/Main/Icons";
import Button from "../../components/Main/Button/Button";

export interface QueryConfiguratorProps {
  error?: ErrorTypes | string;
  queryOptions: string[]
}

const QueryConfigurator: FC<QueryConfiguratorProps> = ({ error, queryOptions }) => {

  const { query, queryHistory, autocomplete } = useQueryState();
  const queryDispatch = useQueryDispatch();
  const timeDispatch = useTimeDispatch();

  const [stateQuery, setStateQuery] = useState(query || []);
  const prevStateQuery = usePrevious(stateQuery) as (undefined | string[]);
  const updateHistory = () => {
    queryDispatch({
      type: "SET_QUERY_HISTORY", payload: stateQuery.map((q, i) => {
        const h = queryHistory[i] || { values: [] };
        const queryEqual = q === h.values[h.values.length - 1];
        return {
          index: h.values.length - Number(queryEqual),
          values: !queryEqual && q ? [...h.values, q] : h.values
        };
      })
    });
  };

  const onRunQuery = () => {
    updateHistory();
    queryDispatch({ type: "SET_QUERY", payload: stateQuery });
    timeDispatch({ type: "RUN_QUERY" });
  };

  const onAddQuery = () => {
    setStateQuery(prev => [...prev, ""]);
  };

  const onRemoveQuery = (index: number) => {
    setStateQuery(prev => prev.filter((q, i) => i !== index));
  };

  const handleChangeQuery = (value: string, index: number) => {
    setStateQuery(prev => prev.map((q, i) => i === index ? value : q));
  };

  const handleHistoryChange = (step: number, indexQuery: number) => {
    const { index, values } = queryHistory[indexQuery];
    const newIndexHistory = index + step;
    if (newIndexHistory < 0 || newIndexHistory >= values.length) return;
    handleChangeQuery(values[newIndexHistory] || "", indexQuery);
    queryDispatch({
      type: "SET_QUERY_HISTORY_BY_INDEX",
      payload: { value: { values, index: newIndexHistory }, queryNumber: indexQuery }
    });
  };

  useEffect(() => {
    if (prevStateQuery && (stateQuery.length < prevStateQuery.filter(q => q).length)) {
      onRunQuery();
    }
  }, [stateQuery]);

  return <div>
    <div>
      {stateQuery.map((q, i) =>
        <div
          key={i}
        >
          <QueryEditor
            value={stateQuery[i]}
            autocomplete={autocomplete}
            options={queryOptions}
            error={error}
            onArrowUp={() => handleHistoryChange(-1, i)}
            onArrowDown={() => handleHistoryChange(1, i)}
            onEnter={onRunQuery}
            onChange={(value) => handleChangeQuery(value, i)}
            label={`Query ${i + 1}`}
            size={"small"}
          />
          {stateQuery.length > 1 && (
            // <Tooltip title="Remove Query">
            <Button
              onClick={() => onRemoveQuery(i)}
              color={"error"}
            >
              <DeleteIcon/>
            </Button>
            // </Tooltip>
          )}
        </div>)}
    </div>
    <div>
      <AdditionalSettings/>
      <div>
        {stateQuery.length < MAX_QUERY_FIELDS && (
          <Button
            variant="outlined"
            onClick={onAddQuery}
            startIcon={<PlusIcon/>}
          >
            <span>Add Query</span>
          </Button>
        )}
        <Button
          variant="contained"
          onClick={onRunQuery}
          startIcon={<PlayIcon/>}
        >
          <span>Execute Query</span>
        </Button>
      </div>
    </div>
  </div>;
};

export default QueryConfigurator;
