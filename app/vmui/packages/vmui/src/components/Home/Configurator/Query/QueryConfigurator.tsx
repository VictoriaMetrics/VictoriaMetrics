import React, {FC, useEffect, useRef, useState} from "preact/compat";
import Box from "@mui/material/Box";
import IconButton from "@mui/material/IconButton";
import Tooltip from "@mui/material/Tooltip";
import Button from "@mui/material/Button";
import QueryEditor from "./QueryEditor";
import {useAppDispatch, useAppState} from "../../../../state/common/StateContext";
import HighlightOffIcon from "@mui/icons-material/HighlightOff";
import AddIcon from "@mui/icons-material/Add";
import PlayCircleOutlineIcon from "@mui/icons-material/PlayCircleOutline";
import AdditionalSettings from "./AdditionalSettings";
import {ErrorTypes} from "../../../../types";
import Paper from "@mui/material/Paper";

export interface QueryConfiguratorProps {
  error?: ErrorTypes | string;
  queryOptions: string[]
}

const QueryConfigurator: FC<QueryConfiguratorProps> = ({error, queryOptions}) => {

  const {query, queryHistory, queryControls: {autocomplete}} = useAppState();
  const dispatch = useAppDispatch();
  const queryRef = useRef(query);
  useEffect(() => {
    queryRef.current = query;
  }, [query]);

  const updateHistory = () => {
    dispatch({
      type: "SET_QUERY_HISTORY", payload: query.map((q, i) => {
        const h = queryHistory[i] || {values: []};
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
    dispatch({type: "SET_QUERY", payload: query});
    dispatch({type: "RUN_QUERY"});
  };

  const onAddQuery = () => dispatch({type: "SET_QUERY", payload: [...queryRef.current, ""]});

  const onRemoveQuery = (index: number) => {
    const newQuery = [...queryRef.current];
    newQuery.splice(index, 1);
    dispatch({type: "SET_QUERY", payload: newQuery});
  };

  const onSetQuery = (value: string, index: number) => {
    const newQuery = [...queryRef.current];
    newQuery[index] = value;
    dispatch({type: "SET_QUERY", payload: newQuery});
  };

  const setHistoryIndex = (step: number, indexQuery: number) => {
    const {index, values} = queryHistory[indexQuery];
    const newIndexHistory = index + step;
    if (newIndexHistory < 0 || newIndexHistory >= values.length) return;
    onSetQuery(values[newIndexHistory] || "", indexQuery);
    dispatch({
      type: "SET_QUERY_HISTORY_BY_INDEX",
      payload: {value: {values, index: newIndexHistory}, queryNumber: indexQuery}
    });
  };
  // boxShadow="rgba(99, 99, 99, 0.2) 0px 2px 8px 0px;" p={4} mb={4} borderRadius="4px"
  return <Box boxShadow="rgba(99, 99, 99, 0.2) 0px 2px 8px 0px;" p={4} m={-4} mb={4} borderRadius="4px">
    <Box>
      {query.map((q, i) =>
        <Box key={i} display="grid" gridTemplateColumns="1fr auto" gap="4px" width="100%"
          mb={i === query.length - 1 ? 0 : 2.5}>
          <QueryEditor query={query[i]} index={i} autocomplete={autocomplete} queryOptions={queryOptions}
            error={error} setHistoryIndex={setHistoryIndex} runQuery={onRunQuery} setQuery={onSetQuery}/>
          {i === 0 && <Tooltip title="Execute Query">
            <IconButton onClick={onRunQuery}>
              <PlayCircleOutlineIcon/>
            </IconButton>
          </Tooltip>}
          {i > 0 && <Tooltip title="Remove Query">
            <IconButton onClick={() => onRemoveQuery(i)}>
              <HighlightOffIcon/>
            </IconButton>
          </Tooltip>}
        </Box>)}
      {query.length < 2 && <Box display="inline-block" mt={2}>
        <Button onClick={onAddQuery} variant="outlined">
          <AddIcon sx={{fontSize: 16, marginRight: "4px"}}/>
          <span style={{lineHeight: 1, paddingTop: "1px"}}>Query</span>
        </Button>
      </Box>}
    </Box>
    <Box mt={3}>
      <AdditionalSettings/>
    </Box>
  </Box>;
};

export default QueryConfigurator;