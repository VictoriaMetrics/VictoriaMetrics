import React, { FC } from "preact/compat";
import Box from "@mui/material/Box";
import FormControlLabel from "@mui/material/FormControlLabel";
import { saveToStorage } from "../../../utils/storage";
import BasicSwitch from "../../../theme/switch";
import StepConfigurator from "./StepConfigurator";
import { useGraphDispatch } from "../../../state/graph/GraphStateContext";
import { getAppModeParams } from "../../../utils/app-mode";
import TenantsConfiguration from "./TenantsConfiguration";
import QueryAutocompleteSwitcher from "../QueryEditor/QueryAutocompleteSwitcher";
import { useCustomPanelDispatch, useCustomPanelState } from "../../../state/customPanel/CustomPanelStateContext";
import { useTimeState } from "../../../state/time/TimeStateContext";

const AdditionalSettings: FC = () => {

  const graphDispatch = useGraphDispatch();
  const { inputTenantID } = getAppModeParams();

  const { nocache, isTracingEnabled } = useCustomPanelState();
  const customPanelDispatch = useCustomPanelDispatch();

  const { period: { step } } = useTimeState();

  const onChangeCache = () => {
    customPanelDispatch({ type: "TOGGLE_NO_CACHE" });
    saveToStorage("NO_CACHE", !nocache);
  };

  const onChangeQueryTracing = () => {
    customPanelDispatch({ type: "TOGGLE_QUERY_TRACING" });
    saveToStorage("QUERY_TRACING", !isTracingEnabled);
  };

  return <Box
    display="flex"
    alignItems="center"
    flexWrap="wrap"
    gap={2}
  >
    <QueryAutocompleteSwitcher/>
    <Box>
      <FormControlLabel
        label="Disable cache"
        sx={{ m: 0 }}
        control={<BasicSwitch
          checked={nocache}
          onChange={onChangeCache}
        />}
      />
    </Box>
    <Box>
      <FormControlLabel
        label="Trace query"
        sx={{ m: 0 }}
        control={<BasicSwitch
          checked={isTracingEnabled}
          onChange={onChangeQueryTracing}
        />}
      />
    </Box>
    <Box ml={2}>
      <StepConfigurator
        defaultStep={step}
        setStep={(value) => {
          graphDispatch({ type: "SET_CUSTOM_STEP", payload: value });
        }}
      />
    </Box>
    {!!inputTenantID && <Box ml={2}><TenantsConfiguration/></Box>}
  </Box>;
};

export default AdditionalSettings;
