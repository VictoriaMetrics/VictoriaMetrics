import React, {FC} from "react";
import {Box, FormControlLabel, Switch} from "@mui/material";
import {saveToStorage} from "../../../../utils/storage";
import {useAppDispatch, useAppState} from "../../../../state/common/StateContext";

const AdditionalSettings: FC = () => {

  const {queryControls: {autocomplete, nocache}} = useAppState();
  const dispatch = useAppDispatch();

  const onChangeAutocomplete = () => {
    dispatch({type: "TOGGLE_AUTOCOMPLETE"});
    saveToStorage("AUTOCOMPLETE", !autocomplete);
  };

  const onChangeCache = () => {
    dispatch({type: "NO_CACHE"});
    saveToStorage("NO_CACHE", !nocache);
  };

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
  </Box>;
};

export default AdditionalSettings;