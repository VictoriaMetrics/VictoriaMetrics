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

  const {queryControls: {autocomplete, nocache, isTracingEnabled}, time: {period: {step}}} = useAppState();
  const dispatch = useAppDispatch();

  const onChangeAutocomplete = () => {
    dispatch({type: "TOGGLE_AUTOCOMPLETE"});
    saveToStorage("AUTOCOMPLETE", !autocomplete);
  };

  const onChangeCache = () => {
    dispatch({type: "NO_CACHE"});
    saveToStorage("NO_CACHE", !nocache);
  };

  const onChangeQueryTracing = () => {
    dispatch({type: "TOGGLE_QUERY_TRACING"});
    saveToStorage("QUERY_TRACING", !isTracingEnabled);
  };

  return <Box display="flex" alignItems="center">
    <Box>
      <FormControlLabel label="Autocomplete"
        control={<BasicSwitch checked={autocomplete} onChange={onChangeAutocomplete}/>}
      />
    </Box>
    <Box ml={2}>
      <FormControlLabel label="Disable cache"
        control={<BasicSwitch checked={nocache} onChange={onChangeCache}/>}
      />
    </Box>
    <Box ml={2}>
      <FormControlLabel label="Trace query"
        control={<BasicSwitch checked={isTracingEnabled} onChange={onChangeQueryTracing} />}
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
