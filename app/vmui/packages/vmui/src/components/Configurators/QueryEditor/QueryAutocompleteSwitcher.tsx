import React, { FC } from "preact/compat";
import Box from "@mui/material/Box";
import FormControlLabel from "@mui/material/FormControlLabel";
import { saveToStorage } from "../../../utils/storage";
import { useAppDispatch, useAppState } from "../../../state/common/StateContext";
import BasicSwitch from "../../../theme/switch";

const QueryAutocompleteSwitcher: FC = () => {

  const { queryControls: { autocomplete } } = useAppState();
  const dispatch = useAppDispatch();

  const onChangeAutocomplete = () => {
    dispatch({ type: "TOGGLE_AUTOCOMPLETE" });
    saveToStorage("AUTOCOMPLETE", !autocomplete);
  };

  return (
    <Box>
      <FormControlLabel
        label="Autocomplete"
        sx={{ m: 0 }}
        control={<BasicSwitch
          checked={autocomplete}
          onChange={onChangeAutocomplete}
        />}
      />
    </Box>
  );
};

export default QueryAutocompleteSwitcher;
