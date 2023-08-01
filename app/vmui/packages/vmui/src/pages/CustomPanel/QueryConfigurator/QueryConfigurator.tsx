import React, { FC, StateUpdater, useEffect, useState } from "preact/compat";
import QueryEditor from "../../../components/Configurators/QueryEditor/QueryEditor";
import AdditionalSettings from "../../../components/Configurators/AdditionalSettings/AdditionalSettings";
import usePrevious from "../../../hooks/usePrevious";
import { MAX_QUERY_FIELDS } from "../../../constants/graph";
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
import { useAppState } from "../../../state/common/StateContext";

export interface QueryConfiguratorProps {
  queryErrors: string[];
  setQueryErrors: StateUpdater<string[]>;
  stats: QueryStats[];
  queryOptions: string[]
  onHideQuery: (queries: number[]) => void
  onRunQuery: () => void
}

const QueryConfigurator: FC<QueryConfiguratorProps> = ({
  queryErrors,
  setQueryErrors,
  stats,
  queryOptions,
  onHideQuery,
  onRunQuery
}) => {

  const { isMobile } = useDeviceDetect();

  const { query, queryHistory, autocomplete } = useQueryState();
  const queryDispatch = useQueryDispatch();
  const timeDispatch = useTimeDispatch();

  const [stateQuery, setStateQuery] = useState(query || []);
  const [hideQuery, setHideQuery] = useState<number[]>([]);
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
  };

  const createHandlerRemoveQuery = (i: number) => () => {
    handleRemoveQuery(i);
    setHideQuery(prev => prev.includes(i) ? prev.filter(n => n !== i) : prev.map(n => n > i ? n - 1: n));
  };

  const createHandlerHideQuery = (i: number) => (e: ReactMouseEvent<HTMLButtonElement, MouseEvent>) => {
    handleToggleHideQuery(e, i);
  };

  const { serverUrl } = useAppState();

  const handlePrettifyQuery = async (el: HTMLButtonElement, i: number) => {

    const oldQuery = encodeURIComponent(stateQuery[i]);
    let response: Response;
    try {
      response = await fetch(`${serverUrl}/prettify-query?query=${oldQuery}`);
    } catch (e) {
      const newQueryErrors = [...queryErrors];
      newQueryErrors[i] = `${e}`;
      setQueryErrors(newQueryErrors);
      return;
    }

    if (response.status != 200) {
      const newQueryErrors = [...queryErrors];
      newQueryErrors[i] = "Error requesting /prettify-query, status: " + response.status;
      setQueryErrors(newQueryErrors);
    }

    const data = await response.json();

    if (data["status"] == "success") {
      const newQueryErrors = [...queryErrors];
      newQueryErrors[i] = "";
      setQueryErrors(newQueryErrors);

      const newStateQuery = [...stateQuery];
      newStateQuery[i] = data["query"];
      setStateQuery(newStateQuery);
    } else {
      const newQueryErrors = [...queryErrors];
      newQueryErrors[i] = data["msg"];
      setQueryErrors(newQueryErrors);
    }

  };

  const createHandlerPrettifyQuery = (i: number) => {
    return (e: ReactMouseEvent<HTMLButtonElement, MouseEvent>) => {
      handlePrettifyQuery(e.currentTarget, i);
    };
  };

  useEffect(() => {
    if (prevStateQuery && (stateQuery.length < prevStateQuery.length)) {
      handleRunQuery();
    }
  }, [stateQuery]);

  useEffect(() => {
    onHideQuery(hideQuery);
  }, [hideQuery]);

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
            autocomplete={autocomplete}
            options={queryOptions}
            error={queryErrors[i]}
            stats={stats[i]}
            onArrowUp={createHandlerArrow(-1, i)}
            onArrowDown={createHandlerArrow(1, i)}
            onEnter={handleRunQuery}
            onChange={createHandlerChangeQuery(i)}
            label={`Query ${i + 1}`}
            disabled={hideQuery.includes(i)}
          />
          <Tooltip title={hideQuery.includes(i) ? "Enable query" : "Disable query"}>
            <div className="vm-query-configurator-list-row__button">
              <Button
                variant={"text"}
                color={"gray"}
                startIcon={hideQuery.includes(i) ? <VisibilityOffIcon/> : <VisibilityIcon/>}
                onClick={createHandlerHideQuery(i)}
              />
            </div>
          </Tooltip>

          <Tooltip title={"Prettify query"}>
            <div className="vm-query-configurator-list-row__button">
              <Button
                variant={"text"}
                color={"gray"}
                startIcon={<Prettify/>}
                onClick={createHandlerPrettifyQuery(i)}
                className="prettify"
              />
            </div>
          </Tooltip>

          {stateQuery.length > 1 && (
            <Tooltip title="Remove Query">
              <div className="vm-query-configurator-list-row__button">
                <Button
                  variant={"text"}
                  color={"error"}
                  startIcon={<DeleteIcon/>}
                  onClick={createHandlerRemoveQuery(i)}
                />
              </div>
            </Tooltip>
          )}
        </div>
      ))}
    </div>
    <div className="vm-query-configurator-settings">
      <AdditionalSettings/>
      <div className="vm-query-configurator-settings__buttons">
        {stateQuery.length < MAX_QUERY_FIELDS && (
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
