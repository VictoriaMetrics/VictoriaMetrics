import React, { FC } from "preact/compat";
import Box from "@mui/material/Box";
import StepConfigurator from "./StepConfigurator";
import { useGraphDispatch } from "../../../state/graph/GraphStateContext";
import { getAppModeParams } from "../../../utils/app-mode";
import TenantsConfiguration from "./TenantsConfiguration";
import { useCustomPanelDispatch, useCustomPanelState } from "../../../state/customPanel/CustomPanelStateContext";
import { useTimeState } from "../../../state/time/TimeStateContext";
import Toggle from "../../Main/Toggle/Toggle";
import { useQueryDispatch, useQueryState } from "../../../state/query/QueryStateContext";

const AdditionalSettings: FC = () => {

  const graphDispatch = useGraphDispatch();
  const { inputTenantID } = getAppModeParams();

  const { autocomplete } = useQueryState();
  const queryDispatch = useQueryDispatch();

  const { nocache, isTracingEnabled } = useCustomPanelState();
  const customPanelDispatch = useCustomPanelDispatch();

  const { period: { step } } = useTimeState();

  const onChangeCache = () => {
    customPanelDispatch({ type: "TOGGLE_NO_CACHE" });
  };

  const onChangeQueryTracing = () => {
    customPanelDispatch({ type: "TOGGLE_QUERY_TRACING" });
  };

  const onChangeAutocomplete = () => {
    queryDispatch({ type: "TOGGLE_AUTOCOMPLETE" });
  };

  const onChangeStep = (value: number) => {
    graphDispatch({ type: "SET_CUSTOM_STEP", payload: value });
  };

  return <Box
    display="flex"
    alignItems="center"
    flexWrap="wrap"
    gap={2}
  >
    <Toggle
      label={"Autocomplete"}
      value={autocomplete}
      onChange={onChangeAutocomplete}
    />
    <Toggle
      label={"Disable cache"}
      value={nocache}
      onChange={onChangeCache}
    />
    <Toggle
      label={"Disable cache"}
      value={nocache}
      onChange={onChangeCache}
    />
    <Toggle
      label={"Trace query"}
      value={isTracingEnabled}
      onChange={onChangeQueryTracing}
    />
    <Box ml={2}>
      <StepConfigurator
        defaultStep={step}
        setStep={onChangeStep}
      />
    </Box>
    {!!inputTenantID && (
      <Box ml={2}>
        <TenantsConfiguration/>
      </Box>
    )}
  </Box>;
};

export default AdditionalSettings;
