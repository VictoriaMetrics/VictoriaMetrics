import React, {FC, useState, useEffect} from "preact/compat";
import Box from "@mui/material/Box";
import IconButton from "@mui/material/IconButton";
import Tooltip from "@mui/material/Tooltip";
import QueryEditor from "./QueryEditor";
import {useAppDispatch, useAppState} from "../../../../state/common/StateContext";
import DeleteIcon from "@mui/icons-material/Delete";
import AddIcon from "@mui/icons-material/Add";
import PlayArrowIcon from "@mui/icons-material/PlayArrow";
import AdditionalSettings from "./AdditionalSettings";
import {ErrorTypes} from "../../../../types";
import Button from "@mui/material/Button";
import Typography from "@mui/material/Typography";
import usePrevious from "../../../../hooks/usePrevious";
import {MAX_QUERY_FIELDS} from "../../../../config";

export interface QueryConfiguratorProps {
  error?: ErrorTypes | string;
  queryOptions: string[]
}


const QueryConfigurator: FC<QueryConfiguratorProps> = ({error, queryOptions}) => {

  const {query, queryHistory, queryControls: {autocomplete}} = useAppState();
  const [stateQuery, setStateQuery] = useState(query || []);
  const prevStateQuery = usePrevious(stateQuery) as (undefined | string[]);
  const dispatch = useAppDispatch();

  const updateHistory = () => {
    dispatch({
      type: "SET_QUERY_HISTORY", payload: stateQuery.map((q, i) => {
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
    dispatch({type: "SET_QUERY", payload: stateQuery});
    dispatch({type: "RUN_QUERY"});
  };

  const onAddQuery = () => {
    setStateQuery(prev => [...prev, ""]);
  };

  const onRemoveQuery = (index: number) => {
    setStateQuery(prev => prev.filter((q, i) => i !== index));
  };

  const onSetQuery = (value: string, index: number) => {
    setStateQuery(prev => prev.map((q, i) => i === index ? value : q));
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

  useEffect(() => {
    if (prevStateQuery && (stateQuery.length < prevStateQuery.filter(q => q).length)) {
      onRunQuery();
    }
  }, [stateQuery]);

  return <Box boxShadow="rgba(99, 99, 99, 0.2) 0px 2px 8px 0px;" p={4} pb={2} m={-4} mb={2}>
    <Box>
      {stateQuery.map((q, i) =>
        <Box key={i} display="grid" gridTemplateColumns="1fr auto" gap="4px" width="100%" position="relative"
          mb={i === stateQuery.length - 1 ? 0 : 2}>
          <QueryEditor
            query={stateQuery[i]} index={i} autocomplete={autocomplete} queryOptions={queryOptions}
            error={error} setHistoryIndex={setHistoryIndex} runQuery={onRunQuery} setQuery={onSetQuery}
            label={`Query ${i + 1}`} size={"small"}/>
          {stateQuery.length > 1 && <Tooltip title="Remove Query">
            <IconButton onClick={() => onRemoveQuery(i)} sx={{height: "33px", width: "33px", padding: 0}} color={"error"}>
              <DeleteIcon fontSize={"small"}/>
            </IconButton>
          </Tooltip>}
        </Box>)}
    </Box>
    <Box mt={3} display="grid" gridTemplateColumns="1fr auto" alignItems="center">
      <AdditionalSettings/>
      <Box>
        {stateQuery.length < MAX_QUERY_FIELDS && (
          <Button variant="outlined" onClick={onAddQuery} startIcon={<AddIcon/>} sx={{mr: 2}}>
            <Typography lineHeight={"20px"} fontWeight="500">Add Query</Typography>
          </Button>
        )}
        <Button variant="contained" onClick={onRunQuery} startIcon={<PlayArrowIcon/>}>
          <Typography lineHeight={"20px"} fontWeight="500">Execute Query</Typography>
        </Button>
      </Box>
    </Box>
  </Box>;
};

export default QueryConfigurator;
