import React, { FC } from "preact/compat";
import StepConfigurator from "../StepConfigurator/StepConfigurator";
import { useGraphDispatch } from "../../../state/graph/GraphStateContext";
import { getAppModeParams } from "../../../utils/app-mode";
import TenantsConfiguration from "../TenantsConfiguration/TenantsConfiguration";
import { useCustomPanelDispatch, useCustomPanelState } from "../../../state/customPanel/CustomPanelStateContext";
import { useTimeState } from "../../../state/time/TimeStateContext";
import { useQueryDispatch, useQueryState } from "../../../state/query/QueryStateContext";
import "./style.scss";
import Switch from "../../Main/Switch/Switch";

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

  return <div className="vm-additional-settings">
    <Switch
      label={"Autocomplete"}
      value={autocomplete}
      onChange={onChangeAutocomplete}
    />
    <Switch
      label={"Disable cache"}
      value={nocache}
      onChange={onChangeCache}
    />
    <Switch
      label={"Disable cache"}
      value={nocache}
      onChange={onChangeCache}
    />
    <Switch
      label={"Trace query"}
      value={isTracingEnabled}
      onChange={onChangeQueryTracing}
    />
    <StepConfigurator
      defaultStep={step}
      setStep={onChangeStep}
    />
    {!!inputTenantID && <TenantsConfiguration/>}
  </div>;
};

export default AdditionalSettings;
