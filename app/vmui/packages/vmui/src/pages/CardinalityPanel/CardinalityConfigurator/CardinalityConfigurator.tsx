import React, { ChangeEvent, FC } from "react";
import Box from "@mui/material/Box";
import QueryEditor from "../../../components/Configurators/QueryEditor/QueryEditor";
import Tooltip from "@mui/material/Tooltip";
import IconButton from "@mui/material/IconButton";
import PlayCircleOutlineIcon from "@mui/icons-material/PlayCircleOutline";
import { useFetchQueryOptions } from "../../../hooks/useFetchQueryOptions";
import TextField from "@mui/material/TextField";
import { ErrorTypes } from "../../../types";
import { useQueryDispatch, useQueryState } from "../../../state/query/QueryStateContext";
import Toggle from "../../../components/Main/Toggle/Toggle";

export interface CardinalityConfiguratorProps {
  onSetHistory: (step: number) => void;
  onSetQuery: (query: string) => void;
  onRunQuery: () => void;
  onTopNChange: (e: ChangeEvent<HTMLTextAreaElement|HTMLInputElement>) => void;
  onFocusLabelChange: (e: ChangeEvent<HTMLTextAreaElement|HTMLInputElement>) => void;
  query: string;
  topN: number;
  error?: ErrorTypes | string;
  totalSeries: number;
  totalLabelValuePairs: number;
  date: string | null;
  match: string | null;
  focusLabel: string | null;
}

const CardinalityConfigurator: FC<CardinalityConfiguratorProps> = ({
  topN,
  error,
  query,
  onSetHistory,
  onRunQuery,
  onSetQuery,
  onTopNChange,
  onFocusLabelChange,
  totalSeries,
  totalLabelValuePairs,
  date,
  match,
  focusLabel
}) => {
  const { autocomplete } = useQueryState();
  const queryDispatch = useQueryDispatch();

  const { queryOptions } = useFetchQueryOptions();

  const onChangeAutocomplete = () => {
    queryDispatch({ type: "TOGGLE_AUTOCOMPLETE" });
  };

  return <Box
    boxShadow="rgba(99, 99, 99, 0.2) 0px 2px 8px 0px;"
    p={4}
    pb={2}
    mb={2}
  >
    <Box>
      <Box
        display="grid"
        gridTemplateColumns="1fr auto auto auto auto"
        gap={2}
        width="100%"
        mb={4}
      >
        <QueryEditor
          value={query || match || ""}
          autocomplete={autocomplete}
          options={queryOptions}
          error={error}
          onArrowUp={() => onSetHistory(-1)}
          onArrowDown={() => onSetHistory(1)}
          onEnter={onRunQuery}
          onChange={(value) => onSetQuery(value)}
          label={"Time series selector"}
        />
        <Box>
          <TextField
            label="Number of entries per table"
            type="number"
            size="medium"
            variant="outlined"
            value={topN}
            error={topN < 1}
            helperText={topN < 1 ? "Number must be bigger than zero" : " "}
            onChange={onTopNChange}
          />
        </Box>
        <Box>
          <TextField
            label="Focus label"
            type="text"
            size="medium"
            variant="outlined"
            value={focusLabel}
            onChange={onFocusLabelChange}
          />
        </Box>
        <Box
          display="flex"
          alignItems="center"
          justifyContent="center"
        >
          <Toggle
            label={"Autocomplete"}
            value={autocomplete}
            onChange={onChangeAutocomplete}
          />
        </Box>
        <Tooltip title="Execute Query">
          <IconButton
            onClick={onRunQuery}
            sx={{ height: "49px", width: "49px" }}
          >
            <PlayCircleOutlineIcon/>
          </IconButton>
        </Tooltip>
      </Box>
    </Box>
    <Box>
      Analyzed <b>{totalSeries}</b> series with <b>{totalLabelValuePairs}</b> &quot;label=value&quot; pairs
      at <b>{date}</b> {match && <span>for series selector <b>{match}</b></span>}.
      Show top {topN} entries per table.
    </Box>
  </Box>;
};

export default CardinalityConfigurator;
