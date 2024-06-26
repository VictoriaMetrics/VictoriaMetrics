import React, { Dispatch, FC, SetStateAction, useEffect, useState } from "preact/compat";
import QueryEditor from "../../../components/Configurators/QueryEditor/QueryEditor";
import AdditionalSettings from "../../../components/Configurators/AdditionalSettings/AdditionalSettings";
import usePrevious from "../../../hooks/usePrevious";
import { MAX_QUERIES_HISTORY, MAX_QUERY_FIELDS } from "../../../constants/graph";
import { useQueryDispatch, useQueryState } from "../../../state/query/QueryStateContext";
import { useTimeDispatch } from "../../../state/time/TimeStateContext";
import {
  DeleteIcon,
  PlayIcon,
  PlusIcon,
  Prettify,
  VisibilityIcon,
  VisibilityOffIcon
} from "../../../components/Main/Icons";
import Button from "../../../components/Main/Button/Button";
import "./style.scss";
import Tooltip from "../../../components/Main/Tooltip/Tooltip";
import classNames from "classnames";
import { MouseEvent as ReactMouseEvent } from "react";
import { arrayEquals } from "../../../utils/array";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import { QueryStats } from "../../../api/types";
import { usePrettifyQuery } from "./hooks/usePrettifyQuery";
import QueryHistory from "../QueryHistory/QueryHistory";
import AnomalyConfig from "../../../components/ExploreAnomaly/AnomalyConfig";

export interface QueryConfiguratorProps {
  queryErrors: string[];
  setQueryErrors: Dispatch<SetStateAction<string[]>>;
  setHideError: Dispatch<SetStateAction<boolean>>;
  stats: QueryStats[];
  onHideQuery?: (queries: number[]) => void
  onRunQuery: () => void;
  hideButtons?: {
    addQuery?: boolean;
    prettify?: boolean;
    autocomplete?: boolean;
    traceQuery?: boolean;
    anomalyConfig?: boolean;
  }
}

const QueryConfigurator: FC<QueryConfiguratorProps> = ({
  queryErrors,
  setQueryErrors,
  setHideError,
  stats,
  onHideQuery,
  onRunQuery,
  hideButtons
}) => {

  const { isMobile } = useDeviceDetect();

  const { query, queryHistory, autocomplete, autocompleteQuick } = useQueryState();
  const queryDispatch = useQueryDispatch();
  const timeDispatch = useTimeDispatch();

  const [stateQuery, setStateQuery] = useState(query || []);
  const [hideQuery, setHideQuery] = useState<number[]>([]);
  const [awaitStateQuery, setAwaitStateQuery] = useState(false);
  const prevStateQuery = usePrevious(stateQuery) as (undefined | string[]);

  const getPrettifiedQuery = usePrettifyQuery();

  const updateHistory = () => {
    queryDispatch({
      type: "SET_QUERY_HISTORY",
      payload: stateQuery.map((q, i) => {
        const h = queryHistory[i] || { values: [] };
        const queryEqual = q === h.values[h.values.length - 1];
        const newValues = !queryEqual && q ? [...h.values, q] : h.values;

        // limit the history
        if (newValues.length > MAX_QUERIES_HISTORY)  newValues.shift();

        return {
          index: h.values.length - Number(queryEqual),
          values: newValues
        };
      })
    });
  };

  const handleRunQuery = () => {
    updateHistory();
    queryDispatch({ type: "SET_QUERY", payload: stateQuery });
    timeDispatch({ type: "RUN_QUERY" });
    onRunQuery();
  };

  const handleAddQuery = () => {
    setStateQuery(prev => [...prev, ""]);
  };

  const handleRemoveQuery = (index: number) => {
    setStateQuery(prev => prev.filter((q, i) => i !== index));
  };

  const handleToggleHideQuery = (e: ReactMouseEvent<HTMLButtonElement, MouseEvent>, index: number) => {
    const { ctrlKey, metaKey } = e;
    const ctrlMetaKey = ctrlKey || metaKey;

    if (ctrlMetaKey) {
      const hideIndexes = stateQuery.map((q, i) => i).filter(n => n !== index);
      setHideQuery(prev => arrayEquals(hideIndexes, prev) ? [] : hideIndexes);
    } else {
      setHideQuery(prev => prev.includes(index) ? prev.filter(n => n !== index) : [...prev, index]);
    }
  };

  const handleChangeQuery = (value: string, index: number) => {
    setStateQuery(prev => prev.map((q, i) => i === index ? value : q));
  };

  const handleSelectHistory = (value: string, index: number) => {
    handleChangeQuery(value, index);
    setAwaitStateQuery(true);
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

  const createHandlerArrow = (step: number, i: number) => () => {
    handleHistoryChange(step, i);
  };

  const createHandlerChangeQuery = (i: number) => (value: string) => {
    handleChangeQuery(value, i);
    queryDispatch({ type: "SET_AUTOCOMPLETE_QUICK", payload: false });
  };

  const createHandlerRemoveQuery = (i: number) => () => {
    handleRemoveQuery(i);
    setHideQuery(prev => prev.includes(i) ? prev.filter(n => n !== i) : prev.map(n => n > i ? n - 1 : n));
  };

  const createHandlerHideQuery = (i: number) => (e: ReactMouseEvent<HTMLButtonElement, MouseEvent>) => {
    handleToggleHideQuery(e, i);
  };

  const handlePrettifyQuery = async (i:number) => {
    const prettyQuery = await getPrettifiedQuery(stateQuery[i]);
    setHideError(false);

    handleChangeQuery(prettyQuery.query, i);

    setQueryErrors((qe) => {
      qe[i] = prettyQuery.error;
      return [...qe];
    });
  };

  useEffect(() => {
    if (prevStateQuery && (stateQuery.length < prevStateQuery.length)) {
      handleRunQuery();
    }
  }, [stateQuery]);

  useEffect(() => {
    onHideQuery && onHideQuery(hideQuery);
  }, [hideQuery]);

  useEffect(() => {
    if (awaitStateQuery) {
      handleRunQuery();
      setAwaitStateQuery(false);
    }
  }, [stateQuery, awaitStateQuery]);

  useEffect(() => {
    setStateQuery(query || []);
  }, [query]);

  return <div
    className={classNames({
      "vm-query-configurator": true,
      "vm-block": true,
      "vm-block_mobile": isMobile
    })}
  >
    <div className="vm-query-configurator-list">
      {stateQuery.map((q, i) => (
        <div
          className={classNames({
            "vm-query-configurator-list-row": true,
            "vm-query-configurator-list-row_disabled": hideQuery.includes(i),
            "vm-query-configurator-list-row_mobile": isMobile
          })}
          key={i}
        >
          <QueryEditor
            value={stateQuery[i]}
            autocomplete={!hideButtons?.autocomplete && (autocomplete || autocompleteQuick)}
            error={queryErrors[i]}
            stats={stats[i]}
            onArrowUp={createHandlerArrow(-1, i)}
            onArrowDown={createHandlerArrow(1, i)}
            onEnter={handleRunQuery}
            onChange={createHandlerChangeQuery(i)}
            label={`Query ${stateQuery.length > 1 ? i + 1 : ""}`}
            disabled={hideQuery.includes(i)}
          />
          {onHideQuery && (
            <Tooltip title={hideQuery.includes(i) ? "Enable query" : "Disable query"}>
              <div className="vm-query-configurator-list-row__button">
                <Button
                  variant={"text"}
                  color={"gray"}
                  startIcon={hideQuery.includes(i) ? <VisibilityOffIcon/> : <VisibilityIcon/>}
                  onClick={createHandlerHideQuery(i)}
                  ariaLabel="visibility query"
                />
              </div>
            </Tooltip>
          )}

          {!hideButtons?.prettify && (
            <Tooltip title={"Prettify query"}>
              <div className="vm-query-configurator-list-row__button">
                <Button
                  variant={"text"}
                  color={"gray"}
                  startIcon={<Prettify/>}
                  onClick={async () => await handlePrettifyQuery(i)}
                  className="prettify"
                  ariaLabel="prettify the query"
                />
              </div>
            </Tooltip>)}

          {stateQuery.length > 1 && (
            <Tooltip title="Remove Query">
              <div className="vm-query-configurator-list-row__button">
                <Button
                  variant={"text"}
                  color={"error"}
                  startIcon={<DeleteIcon/>}
                  onClick={createHandlerRemoveQuery(i)}
                  ariaLabel="remove query"
                />
              </div>
            </Tooltip>
          )}
        </div>
      ))}
    </div>
    <div className="vm-query-configurator-settings">
      <AdditionalSettings hideButtons={hideButtons}/>
      <div className="vm-query-configurator-settings__buttons">
        <QueryHistory handleSelectQuery={handleSelectHistory}/>
        {hideButtons?.anomalyConfig && <AnomalyConfig/>}
        {!hideButtons?.addQuery && stateQuery.length < MAX_QUERY_FIELDS && (
          <Button
            variant="outlined"
            onClick={handleAddQuery}
            startIcon={<PlusIcon/>}
          >
            Add Query
          </Button>
        )}
        <Button
          variant="contained"
          onClick={handleRunQuery}
          startIcon={<PlayIcon/>}
        >
          {isMobile ? "Execute" : "Execute Query"}
        </Button>
      </div>
    </div>
  </div>;
};

export default QueryConfigurator;
