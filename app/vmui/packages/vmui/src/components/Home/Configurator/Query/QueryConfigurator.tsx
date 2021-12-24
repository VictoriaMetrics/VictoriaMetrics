import React, {FC, useEffect, useRef, useState} from "preact/compat";
import {
  Accordion, AccordionDetails, AccordionSummary, Box, IconButton, Typography, Tooltip, Button, Portal
} from "@mui/material";
import QueryEditor from "./QueryEditor";
import {TimeSelector} from "../Time/TimeSelector";
import {useAppDispatch, useAppState} from "../../../../state/common/StateContext";
import ExpandMoreIcon from "@mui/icons-material/ExpandMore";
import HighlightOffIcon from "@mui/icons-material/HighlightOff";
import AddIcon from "@mui/icons-material/Add";
import PlayCircleOutlineIcon from "@mui/icons-material/PlayCircleOutline";
import ServerConfigurator from "./ServerConfigurator";
import AdditionalSettings from "./AdditionalSettings";
import {ErrorTypes} from "../../../../types";

export interface QueryConfiguratorProps {
    error?: ErrorTypes | string;
}

const QueryConfigurator: FC<QueryConfiguratorProps> = ({error}) => {

  const {serverUrl, query, queryHistory, queryControls: {autocomplete}} = useAppState();
  const dispatch = useAppDispatch();
  const [expanded, setExpanded] = useState(true);
  const queryContainer = useRef<HTMLDivElement>(null);
  const queryRef = useRef(query);
  useEffect(() => {
    queryRef.current = query;
  }, [query]);

  const onSetDuration = (dur: string) => dispatch({type: "SET_DURATION", payload: dur});

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

  return <>
    <Accordion expanded={expanded} onChange={() => setExpanded(prev => !prev)}>
      <AccordionSummary
        expandIcon={<IconButton><ExpandMoreIcon/></IconButton>}
        aria-controls="panel1a-content"
        id="panel1a-header"
        sx={{alignItems: "flex-start", padding: "15px"}}
      >
        <Box mr={2}>
          <Typography variant="h6" component="h2">Query Configuration</Typography>
        </Box>
        <Box flexGrow={1} onClick={e => e.stopPropagation()} onFocusCapture={e => e.stopPropagation()}>
          <Portal disablePortal={!expanded} container={queryContainer.current}>
            {query.map((q, i) =>
              <Box key={i} display="grid" gridTemplateColumns="1fr auto" gap="4px" width="100%"
                mb={i === query.length - 1 ? 0 : 2}>
                <QueryEditor server={serverUrl} query={query[i]} index={i} oneLiner={!expanded}
                  autocomplete={autocomplete} queryHistory={queryHistory[i]} error={error}
                  setHistoryIndex={setHistoryIndex} runQuery={onRunQuery}
                  setQuery={onSetQuery}/>
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
          </Portal>
        </Box>
      </AccordionSummary>
      <AccordionDetails>
        <Box display="flex" flexWrap="wrap" gap={2}>
          <Box flexGrow="2" minWidth="50%">
            <ServerConfigurator error={error}/>
            {/* for portal QueryEditor */}
            <div ref={queryContainer}/>
            {query.length < 2 && <Box display="inline-block" minHeight="40px" mt={2}>
              <Button onClick={onAddQuery} variant="outlined">
                <AddIcon sx={{fontSize: 16, marginRight: "4px"}}/>
                <span style={{lineHeight: 1, paddingTop: "1px"}}>Query</span>
              </Button>
            </Box>}
          </Box>
          <Box flexGrow="1">
            <TimeSelector setDuration={onSetDuration}/>
          </Box>
          <Box flexBasis="100%" pt={1}>
            <AdditionalSettings/>
          </Box>
        </Box>
      </AccordionDetails>
    </Accordion>
  </>;
};

export default QueryConfigurator;