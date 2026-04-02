import { Dispatch, FC, SetStateAction, useEffect, useState } from "preact/compat";
import QueryEditor from "../../../components/Configurators/QueryEditor/QueryEditor";
import AdditionalSettings from "../../../components/Configurators/AdditionalSettings/AdditionalSettings";
import usePrevious from "../../../hooks/usePrevious";
import { MAX_QUERY_FIELDS } from "../../../constants/graph";
import { useQueryDispatch, useQueryState } from "../../../state/query/QueryStateContext";
import { useTimeDispatch } from "../../../state/time/TimeStateContext";
import { getQueryStringValue } from "../../../utils/query-string";
import {
  DeleteIcon,
  PlayIcon,
  PlusIcon,
  Prettify,
  SpinnerIcon,
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
import useSearchParamsFromObject from "../../../hooks/useSearchParamsFromObject";
import { QueryStats } from "../../../api/types";
import { usePrettifyQuery } from "./hooks/usePrettifyQuery";
import QueryHistory from "../../../components/QueryHistory/QueryHistory";
import QueryEditorAutocomplete from "../../../components/Configurators/QueryEditor/QueryEditorAutocomplete";
import { getUpdatedHistory } from "../../../components/QueryHistory/utils";

export interface QueryConfiguratorProps {
  queryErrors?: string[];
  setQueryErrors: Dispatch<SetStateAction<string[]>>;
  setHideError: Dispatch<SetStateAction<boolean>>;
  stats: QueryStats[];
  label?: string;
  isLoading?: boolean;
  includeFunctions?: boolean;
  onHideQuery?: (queries: number[]) => void
  onRunQuery: () => void;
  abortFetch?: () => void;
  hideButtons?: {
    addQuery?: boolean;
    prettify?: boolean;
    autocomplete?: boolean;
    traceQuery?: boolean;
    disableCache?: boolean;
    reduceMemUsage?: boolean;
  }
}

const defaultHideQueryStr = getQueryStringValue("expr.hide", "") as string;
const defaultHideQuery: number[] = defaultHideQueryStr.split(",").filter(v => v).map(Number);

const QueryConfigurator: FC<QueryConfiguratorProps> = ({
  queryErrors,
  setQueryErrors,
  setHideError,
  stats,
  label,
  isLoading,
  includeFunctions = true,
  onHideQuery,
  onRunQuery,
  abortFetch,
  hideButtons
}) => {

  const { isMobile } = useDeviceDetect();

  const { query, queryHistory, autocomplete, autocompleteQuick } = useQueryState();
  const queryDispatch = useQueryDispatch();
  const timeDispatch = useTimeDispatch();
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();

  const [stateQuery, setStateQuery] = useState(query || []);
  const [hideQuery, setHideQuery] = useState<number[]>(defaultHideQuery);
  const [awaitStateQuery, setAwaitStateQuery] = useState(false);
  const prevStateQuery = usePrevious(stateQuery) as (undefined | string[]);

  const getPrettifiedQuery = usePrettifyQuery();

  const updateHistory = () => {
    queryDispatch({
      type: "SET_QUERY_HISTORY",
      payload: {
        key: "METRICS_QUERY_HISTORY",
        history: stateQuery.map((q, i) => getUpdatedHistory(q, queryHistory[i]))
      }
    });
  };

  const handleRunQuery = () => {
    if (isLoading) {
      abortFetch && abortFetch();
      return;
    }
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

  const handleToggleHideQuery = (e: ReactMouseEvent<HTMLButtonElement>, index: number) => {
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

  const createHandlerHideQuery = (i: number) => (e: ReactMouseEvent<HTMLButtonElement>) => {
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
    setSearchParamsFromKeys({ "expr.hide": hideQuery.join(",") });
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
            autocompleteEl={QueryEditorAutocomplete}
            error={queryErrors && queryErrors[i]}
            stats={stats[i]}
            onArrowUp={createHandlerArrow(-1, i)}
            onArrowDown={createHandlerArrow(1, i)}
            onEnter={handleRunQuery}
            onChange={createHandlerChangeQuery(i)}
            label={`${label || "Query"} ${stateQuery.length > 1 ? i + 1 : ""}`}
            disabled={hideQuery.includes(i)}
            includeFunctions={includeFunctions}
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
        <QueryHistory
          handleSelectQuery={handleSelectHistory}
          historyKey={"METRICS_QUERY_HISTORY"}
        />
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
          startIcon={isLoading ? <SpinnerIcon/> : <PlayIcon/>}
        >
          {`${isLoading ? "Cancel" : "Execute"} ${isMobile ? "" : "Query"}`}
        </Button>
      </div>
    </div>
  </div>;
};

export default QueryConfigurator;
