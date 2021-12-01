import React, {FC, useRef, useState} from "react";
import {
  Accordion, AccordionDetails, AccordionSummary, Box, Grid, IconButton, Typography, Tooltip, Button
} from "@mui/material";
import QueryEditor from "./QueryEditor";
import {TimeSelector} from "../Time/TimeSelector";
import {useAppDispatch, useAppState} from "../../../../state/common/StateContext";
import ExpandMoreIcon from "@mui/icons-material/ExpandMore";
import HighlightOffIcon from "@mui/icons-material/HighlightOff";
import AddIcon from "@mui/icons-material/Add";
import PlayCircleOutlineIcon from "@mui/icons-material/PlayCircleOutline";
import Portal from "@mui/material/Portal";
import ServerConfigurator from "./ServerConfigurator";
import AdditionalSettings from "./AdditionalSettings";

const QueryConfigurator: FC = () => {

  const {serverUrl, query, queryHistory, time: {duration}, queryControls: {autocomplete}} = useAppState();
  const dispatch = useAppDispatch();
  const [expanded, setExpanded] = useState(true);
  const [queryString, _setQueryString] = useState(query);
  const queryStringRef = useRef(queryString);
  const queryContainer = useRef<HTMLDivElement>(null);

  const setQueryString = (data: string[]) => {
    queryStringRef.current = data;
    _setQueryString(data);
  };

  const onSetDuration = (dur: string) => dispatch({type: "SET_DURATION", payload: dur});

  const onRunQuery = () => {
    const history = queryHistory.map((h, i) => {
      const lastQueryEqual = queryString[i] === h.values[h.values.length - 1];
      return {
        index: h.values.length - Number(lastQueryEqual),
        values: lastQueryEqual ? h.values : [...h.values, queryString[i]]
      };
    });
    dispatch({type: "RUN_QUERY"});
    dispatch({type: "SET_QUERY_HISTORY", payload: history});
    dispatch({type: "SET_QUERY", payload: queryString});
  };

  const onAddQuery = () => {
    const value = [...queryString, ""];
    setQueryString(value);
    dispatch({type: "SET_QUERY", payload: value});
  };

  const onRemoveQuery = (index: number) => {
    const value = [...queryString];
    value.splice(index, 1);
    setQueryString(value);
    dispatch({type: "SET_QUERY", payload: value});
  };

  const onSetQuery = (value: string, index: number) => {
    const newQuery = [...queryStringRef.current];
    newQuery[index] = value;
    setQueryString(newQuery);
  };

  const setHistoryIndex = (step: number, indexQuery: number) => {
    const {index, values} = queryHistory[indexQuery];
    const newIndexHistory = index + step;
    if (newIndexHistory < 0 || newIndexHistory >= values.length) return;
    const newQuery = values[newIndexHistory] || "";
    onSetQuery(newQuery, indexQuery);
    dispatch({
      type: "SET_QUERY_HISTORY_BY_INDEX",
      payload: {value: {values, index: newIndexHistory}, queryNumber: indexQuery}
    });
  };

  return <>
    <Accordion expanded={expanded} onChange={() => setExpanded(prev => !prev)}>
      <AccordionSummary
        expandIcon={<ExpandMoreIcon/>}
        aria-controls="panel1a-content"
        id="panel1a-header">
        <Box display="flex" alignItems="center" mr={2}>
          <Typography variant="h6" component="h2">Query Configuration</Typography>
        </Box>
        <Box flexGrow={1} onClick={e => e.stopPropagation()} onFocusCapture={e => e.stopPropagation()}>
          <Portal disablePortal={!expanded} container={queryContainer.current}>
            {query.map((q, i) =>
              <Box key={`${i}_${q}`} display="grid" gridTemplateColumns="1fr auto" width="100%" mb={2}>
                <QueryEditor server={serverUrl} query={queryString[i]} index={i} oneLiner={!expanded}
                  autocomplete={autocomplete}
                  queryHistory={queryHistory[i]} setHistoryIndex={setHistoryIndex}
                  runQuery={onRunQuery}
                  setQuery={onSetQuery}/>
                {i === 0 && <Tooltip title="Execute Query">
                  <IconButton onClick={onRunQuery} size="large"
                    sx={{marginTop: !expanded ? "0" : "4.43px"}}>
                    <PlayCircleOutlineIcon/>
                  </IconButton>
                </Tooltip>}
                {i > 0 && <Tooltip title="Remove Query">
                  <IconButton onClick={() => onRemoveQuery(i)} size="large"
                    sx={{marginTop: !expanded ? "0" : "4.43px"}}>
                    <HighlightOffIcon/>
                  </IconButton>
                </Tooltip>}
              </Box>)}
            {query.length < 2 && <Tooltip title="Add Second Query">
              <Button onClick={onAddQuery} variant="outlined">
                <AddIcon sx={{fontSize: 18, marginRight: "6px"}}/>
                <span>Query</span>
              </Button>
            </Tooltip>}
          </Portal>
        </Box>
      </AccordionSummary>
      <AccordionDetails>
        <Grid container spacing={2}>
          <Grid item xs={6} minWidth={400}>
            <ServerConfigurator/>
            {/* for portal QueryEditor */}
            <div ref={queryContainer}/>
          </Grid>
          <Grid item xs>
            <TimeSelector setDuration={onSetDuration} duration={duration}/>
          </Grid>
          <Grid item xs={12}>
            <AdditionalSettings/>
          </Grid>
        </Grid>
      </AccordionDetails>
    </Accordion>
  </>;
};

export default QueryConfigurator;