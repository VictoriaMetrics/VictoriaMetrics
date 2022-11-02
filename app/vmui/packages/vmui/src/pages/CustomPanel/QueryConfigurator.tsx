import React, { FC, useState, useEffect } from "preact/compat";
import Box from "@mui/material/Box";
import IconButton from "@mui/material/IconButton";
import Tooltip from "@mui/material/Tooltip";
import QueryEditor from "../../components/Configurators/QueryEditor/QueryEditor";
import DeleteIcon from "@mui/icons-material/Delete";
import AddIcon from "@mui/icons-material/Add";
import PlayArrowIcon from "@mui/icons-material/PlayArrow";
import AdditionalSettings from "../../components/Configurators/AdditionalSettings/AdditionalSettings";
import { ErrorTypes } from "../../types";
import Button from "@mui/material/Button";
import Typography from "@mui/material/Typography";
import usePrevious from "../../hooks/usePrevious";
import { MAX_QUERY_FIELDS } from "../../config";
import { useQueryDispatch, useQueryState } from "../../state/query/QueryStateContext";
import { useTimeDispatch } from "../../state/time/TimeStateContext";

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

  return <Box>
    <Box>
      {stateQuery.map((q, i) =>
        <Box
          key={i}
          display="grid"
          gridTemplateColumns="1fr auto"
          gap="4px"
          width="100%"
          position="relative"
          mb={i === stateQuery.length - 1 ? 0 : 2}
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
          {stateQuery.length > 1 && <Tooltip title="Remove Query">
            <IconButton
              onClick={() => onRemoveQuery(i)}
              sx={{ height: "33px", width: "33px", padding: 0 }}
              color={"error"}
            >
              <DeleteIcon fontSize={"small"}/>
            </IconButton>
          </Tooltip>}
        </Box>)}
    </Box>
    <Box
      mt={3}
      display="grid"
      gridTemplateColumns="1fr auto"
      alignItems="start"
      gap={4}
    >
      <AdditionalSettings/>
      <Box
        display="grid"
        gridTemplateColumns="repeat(2, auto)"
        gap={1}
      >
        {stateQuery.length < MAX_QUERY_FIELDS && (
          <Button
            variant="outlined"
            onClick={onAddQuery}
            startIcon={<AddIcon/>}
          >
            <Typography
              lineHeight={"20px"}
              fontWeight="500"
            >Add Query</Typography>
          </Button>
        )}
        <Button
          variant="contained"
          onClick={onRunQuery}
          startIcon={<PlayArrowIcon/>}
        >
          <Typography
            lineHeight={"20px"}
            fontWeight="500"
          >Execute Query</Typography>
        </Button>
      </Box>
    </Box>
  </Box>;
};

export default QueryConfigurator;
