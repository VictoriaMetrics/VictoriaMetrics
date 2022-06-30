import React, {ChangeEvent, FC} from "react";
import Box from "@mui/material/Box";
import QueryEditor from "../../CustomPanel/Configurator/Query/QueryEditor";
import Tooltip from "@mui/material/Tooltip";
import IconButton from "@mui/material/IconButton";
import PlayCircleOutlineIcon from "@mui/icons-material/PlayCircleOutline";
import {useFetchQueryOptions} from "../../../hooks/useFetchQueryOptions";
import {useAppDispatch, useAppState} from "../../../state/common/StateContext";
import FormControlLabel from "@mui/material/FormControlLabel";
import BasicSwitch from "../../../theme/switch";
import {saveToStorage} from "../../../utils/storage";
import TextField from "@mui/material/TextField";
import {ErrorTypes} from "../../../types";

export interface CardinalityConfiguratorProps {
  onSetHistory: (step: number, index: number) => void;
  onSetQuery: (query: string, index: number) => void;
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
  const dispatch = useAppDispatch();
  const {queryControls: {autocomplete}} = useAppState();
  const {queryOptions} = useFetchQueryOptions();

  const onChangeAutocomplete = () => {
    dispatch({type: "TOGGLE_AUTOCOMPLETE"});
    saveToStorage("AUTOCOMPLETE", !autocomplete);
  };

  return <Box boxShadow="rgba(99, 99, 99, 0.2) 0px 2px 8px 0px;" p={4} pb={2} mb={2}>
    <Box>
      <Box display="grid" gridTemplateColumns="1fr auto auto auto auto" gap="4px" width="100%" mb={4}>
        <QueryEditor
          query={query} index={0} autocomplete={autocomplete} queryOptions={queryOptions}
          error={error} setHistoryIndex={onSetHistory} runQuery={onRunQuery} setQuery={onSetQuery}
          label={"Time series selector"}
        />
        <Box mr={2}>
          <TextField
            label="Number of entries per table"
            type="number"
            size="medium"
            variant="outlined"
            value={topN}
            error={topN < 1}
            helperText={topN < 1 ? "Number must be bigger than zero" : " "}
            onChange={onTopNChange}/>
        </Box>
        <Box mr={2}>
          <TextField
            label="Focus label"
            type="text"
            size="medium"
            variant="outlined"
            value={focusLabel}
            onChange={onFocusLabelChange} />
        </Box>
        <Box>
          <FormControlLabel label="Autocomplete"
            control={<BasicSwitch checked={autocomplete} onChange={onChangeAutocomplete}/>}
          />
        </Box>
        <Tooltip title="Execute Query">
          <IconButton onClick={onRunQuery} sx={{height: "49px", width: "49px"}}>
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
