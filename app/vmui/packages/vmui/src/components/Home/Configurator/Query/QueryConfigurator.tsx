import React, {FC, useRef, useState} from "react";
import { Accordion,  AccordionDetails,  AccordionSummary,  Box,  Grid,  IconButton, Typography, Tooltip,
  Button } from "@mui/material";
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
  const queryContainer = useRef<HTMLDivElement>(null);

  const onSetDuration = (dur: string) => dispatch({type: "SET_DURATION", payload: dur});
  
  const onRunQuery = () => {
    const { values } = queryHistory;
    dispatch({type: "RUN_QUERY"});
    dispatch({type: "SET_QUERY_HISTORY_INDEX", payload: values.length});
    dispatch({type: "SET_QUERY_HISTORY_VALUES", payload: [...values, ...query]});
  };

  const onAddQuery = () => {
    dispatch({type: "SET_QUERY", payload: [...query, ""]});
  };

  const onRemoveQuery = () => {
    dispatch({type: "SET_QUERY", payload: [query[0]]});
  };

  const onSetQuery = (queryString: string, index: number) => {
    const newQuery = [...query];
    newQuery[index] = queryString;
    dispatch({type: "SET_QUERY", payload: newQuery});
  };

  const setHistoryIndex = (step: number, indexQuery: number) => {
    const indexHistory = queryHistory.index + step;
    if (indexHistory < -1 || indexHistory > queryHistory.values.length) return;
    dispatch({type: "SET_QUERY_HISTORY_INDEX", payload: indexHistory});
    onSetQuery(queryHistory.values[indexHistory] || "", indexQuery);
  };

  return <>
    <Accordion expanded={expanded} onChange={() => setExpanded(prev => !prev)}>
      <AccordionSummary
        expandIcon={<ExpandMoreIcon/>}
        aria-controls="panel1a-content"
        id="panel1a-header"
      >
        <Box display="flex" alignItems="center" mr={2}>
          <Typography variant="h6" component="h2">Query Configuration</Typography>
        </Box>
        <Box flexGrow={1} onClick={e => e.stopPropagation()} onFocusCapture={e => e.stopPropagation()}>
          <Portal disablePortal={!expanded} container={queryContainer.current}>
            <Box display="flex" alignItems={!expanded ? "center" : "start"} mb={2}>
              <Box width="100%">
                <QueryEditor server={serverUrl} query={query} index={0} oneLiner={!expanded} autocomplete={autocomplete}
                  queryHistory={queryHistory} setHistoryIndex={setHistoryIndex} runQuery={onRunQuery}
                  setQuery={onSetQuery}/>
              </Box>
              <Tooltip title="Execute Query">
                <IconButton onClick={onRunQuery} size="large" sx={{marginTop: !expanded ? "0" : "4.43px"}}>
                  <PlayCircleOutlineIcon/>
                </IconButton>
              </Tooltip>
            </Box>
            {query.length > 1 && <Box display="flex" alignItems={!expanded ? "center" : "start"}>
              <Box width="100%">
                <QueryEditor server={serverUrl} query={query} index={1} oneLiner={!expanded} autocomplete={autocomplete}
                  queryHistory={queryHistory} setHistoryIndex={setHistoryIndex} runQuery={onRunQuery}
                  setQuery={onSetQuery}/>
              </Box>
              <Tooltip title="Remove Query">
                <IconButton onClick={onRemoveQuery} size="large" sx={{marginTop: !expanded ? "0" : "4.43px"}}>
                  <HighlightOffIcon/>
                </IconButton>
              </Tooltip>
            </Box>}
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
            <div ref={queryContainer} />{/* for portal QueryEditor */}
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