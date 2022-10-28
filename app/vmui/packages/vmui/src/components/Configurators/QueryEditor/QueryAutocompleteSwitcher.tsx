import React, { FC } from "preact/compat";
import Box from "@mui/material/Box";
import FormControlLabel from "@mui/material/FormControlLabel";
import { saveToStorage } from "../../../utils/storage";
import BasicSwitch from "../../../theme/switch";
import { useQueryDispatch, useQueryState } from "../../../state/query/QueryStateContext";

const QueryAutocompleteSwitcher: FC = () => {

  const { autocomplete } = useQueryState();
  const dispatch = useQueryDispatch();

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
