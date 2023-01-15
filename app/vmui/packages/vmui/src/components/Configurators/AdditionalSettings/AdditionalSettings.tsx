import React, { FC } from "preact/compat";
import { getAppModeParams } from "../../../utils/app-mode";
import TenantsConfiguration from "../TenantsConfiguration/TenantsConfiguration";
import { useCustomPanelDispatch, useCustomPanelState } from "../../../state/customPanel/CustomPanelStateContext";
import { useQueryDispatch, useQueryState } from "../../../state/query/QueryStateContext";
import "./style.scss";
import Switch from "../../Main/Switch/Switch";

const AdditionalSettings: FC = () => {

  const { inputTenantID } = getAppModeParams();

  const { autocomplete } = useQueryState();
  const queryDispatch = useQueryDispatch();

  const { nocache, isTracingEnabled } = useCustomPanelState();
  const customPanelDispatch = useCustomPanelDispatch();

  const onChangeCache = () => {
    customPanelDispatch({ type: "TOGGLE_NO_CACHE" });
  };

  const onChangeQueryTracing = () => {
    customPanelDispatch({ type: "TOGGLE_QUERY_TRACING" });
  };

  const onChangeAutocomplete = () => {
    queryDispatch({ type: "TOGGLE_AUTOCOMPLETE" });
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
      label={"Trace query"}
      value={isTracingEnabled}
      onChange={onChangeQueryTracing}
    />
    {!!inputTenantID && (
      <div className="vm-additional-settings__input">
        <TenantsConfiguration/>
      </div>
    )}
  </div>;
};

export default AdditionalSettings;
