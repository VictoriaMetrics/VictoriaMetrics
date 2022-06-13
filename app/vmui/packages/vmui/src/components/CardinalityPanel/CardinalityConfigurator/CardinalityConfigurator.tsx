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
  query: string;
  topN: number;
  error?: ErrorTypes | string;
  totalSeries: number;
  totalLabelValuePairs: number;
  date: string | null;
  match: string | null;
}

const CardinalityConfigurator: FC<CardinalityConfiguratorProps> = ({
  topN,
  error,
  query,
  onSetHistory,
  onRunQuery,
  onSetQuery,
  onTopNChange,
  totalSeries,
  totalLabelValuePairs,
  date,
  match
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
      <Box display="grid" gridTemplateColumns="1fr auto auto" gap="4px" width="100%" mb={4}>
        <QueryEditor
          query={query} index={0} autocomplete={autocomplete} queryOptions={queryOptions}
          error={error} setHistoryIndex={onSetHistory} runQuery={onRunQuery} setQuery={onSetQuery}
          label={"Arbitrary time series selector"}
        />
        <Box display="flex" alignItems="center">
          <Box ml={2}>
            <TextField
              label="Number of top entries"
              type="number"
              size="small"
              variant="outlined"
              value={topN}
              error={topN < 1}
              helperText={topN < 1 ? "Number must be bigger than zero" : " "}
              onChange={onTopNChange}/>
          </Box>
          <Tooltip title="Execute Query">
            <IconButton onClick={onRunQuery} sx={{height: "49px", width: "49px"}}>
              <PlayCircleOutlineIcon/>
            </IconButton>
          </Tooltip>
          <Box>
            <FormControlLabel label="Enable autocomplete"
              control={<BasicSwitch checked={autocomplete} onChange={onChangeAutocomplete}/>}
            />
          </Box>
        </Box>
      </Box>
    </Box>
    <Box>
      Analyzed <b>{totalSeries}</b> series and <b>{totalLabelValuePairs}</b> label=value pairs
      at <b>{date}</b> {match && <span>for series selector <b>{match}</b></span>}. Show top {topN} entries per table.
    </Box>
  </Box>;
};

export default CardinalityConfigurator;
