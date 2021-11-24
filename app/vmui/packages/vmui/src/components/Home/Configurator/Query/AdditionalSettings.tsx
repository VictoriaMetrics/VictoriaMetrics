import React, {FC} from "react";
import {Box, FormControlLabel, Switch, TextField} from "@mui/material";
import debounce from "lodash.debounce";
import {saveToStorage} from "../../../../utils/storage";
import {useGraphDispatch, useGraphState} from "../../../../state/graph/GraphStateContext";
import {useAppDispatch, useAppState} from "../../../../state/common/StateContext";

const AdditionalSettings: FC = () => {

  const {queryControls: {autocomplete, nocache}} = useAppState();
  const dispatch = useAppDispatch();

  const { yaxis } = useGraphState();
  const graphDispatch = useGraphDispatch();

  const setMinLimit = ({target: {value}}: {target: {value: string}}) => {
    graphDispatch({type: "SET_YAXIS_LIMITS", payload: [+value, yaxis.limits.range[1]]});
  };
  const setMaxLimit = ({target: {value}}: {target: {value: string}}) => {
    graphDispatch({type: "SET_YAXIS_LIMITS", payload: [yaxis.limits.range[0], +value]});
  };

  const onChangeAutocomplete = () => {
    dispatch({type: "TOGGLE_AUTOCOMPLETE"});
    saveToStorage("AUTOCOMPLETE", !autocomplete);
  };

  const onChangeCache = () => {
    dispatch({type: "NO_CACHE"});
    saveToStorage("NO_CACHE", !nocache);
  };

  const onChangeYaxisLimits = () => { graphDispatch({type: "TOGGLE_ENABLE_YAXIS_LIMITS"}); };

  return <Box px={1} display="flex" alignItems="center">
    <Box>
      <FormControlLabel label="Enable autocomplete"
        control={<Switch size="small" checked={autocomplete} onChange={onChangeAutocomplete}/>}
      />
    </Box>
    <Box ml={2}>
      <FormControlLabel label="Enable cache"
        control={<Switch size="small" checked={!nocache} onChange={onChangeCache}/>}
      />
    </Box>
    <Box ml={2} display="flex" alignItems="center" minHeight={52}>
      <FormControlLabel
        control={<Switch size="small" checked={yaxis.limits.enable} onChange={onChangeYaxisLimits}/>}
        label="Fix the limits for y-axis"
      />
      {yaxis.limits.enable && <Box display="grid" gridTemplateColumns="120px 120px" gap={1}>
        <TextField label="Min" type="number" size="small" variant="outlined"
          defaultValue={yaxis.limits.range[0]} onChange={debounce(setMinLimit, 750)}/>
        <TextField label="Max" type="number" size="small" variant="outlined"
          defaultValue={yaxis.limits.range[1]} onChange={debounce(setMaxLimit, 750)}/>
      </Box>}
    </Box>
  </Box>;
};

export default AdditionalSettings;