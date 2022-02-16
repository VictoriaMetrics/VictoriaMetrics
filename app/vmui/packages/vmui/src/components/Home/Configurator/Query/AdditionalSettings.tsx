import React, {FC} from "preact/compat";
import Box from "@mui/material/Box";
import FormControlLabel from "@mui/material/FormControlLabel";
import {saveToStorage} from "../../../../utils/storage";
import {useAppDispatch, useAppState} from "../../../../state/common/StateContext";
import BasicSwitch from "../../../../theme/switch";
import StepConfigurator from "./StepConfigurator";
import {useGraphDispatch, useGraphState} from "../../../../state/graph/GraphStateContext";

const AdditionalSettings: FC = () => {

  const {customStep} = useGraphState();
  const graphDispatch = useGraphDispatch();

  const {queryControls: {autocomplete, nocache}, time: {period: {step}}} = useAppState();
  const dispatch = useAppDispatch();

  const onChangeAutocomplete = () => {
    dispatch({type: "TOGGLE_AUTOCOMPLETE"});
    saveToStorage("AUTOCOMPLETE", !autocomplete);
  };

  const onChangeCache = () => {
    dispatch({type: "NO_CACHE"});
    saveToStorage("NO_CACHE", !nocache);
  };

  return <Box display="flex" alignItems="center">
    <Box>
      <FormControlLabel label="Enable autocomplete"
        control={<BasicSwitch checked={autocomplete} onChange={onChangeAutocomplete}/>}
      />
    </Box>
    <Box ml={2}>
      <FormControlLabel label="Enable cache"
        control={<BasicSwitch checked={!nocache} onChange={onChangeCache}/>}
      />
    </Box>
    <Box ml={2}>
      <StepConfigurator defaultStep={step} customStepEnable={customStep.enable}
        setStep={(value) => {
          graphDispatch({type: "SET_CUSTOM_STEP", payload: value});
        }}
        toggleEnableStep={() => {
          graphDispatch({type: "TOGGLE_CUSTOM_STEP"});
        }}/>
    </Box>
  </Box>;
};

export default AdditionalSettings;