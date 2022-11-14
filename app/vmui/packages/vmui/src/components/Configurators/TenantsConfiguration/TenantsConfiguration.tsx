import React, { FC, useState, useEffect, useCallback } from "preact/compat";
import { useAppDispatch, useAppState } from "../../../state/common/StateContext";
import debounce from "lodash.debounce";
import { getAppModeParams } from "../../../utils/app-mode";
import { useTimeDispatch } from "../../../state/time/TimeStateContext";
import { InfoIcon } from "../../Main/Icons";
import TextField from "../../Main/TextField/TextField";
import Button from "../../Main/Button/Button";
import Tooltip from "../../Main/Tooltip/Tooltip";

const TenantsConfiguration: FC = () => {
  const { serverURL } = getAppModeParams();
  const { tenantId: tenantIdState } = useAppState();
  const dispatch = useAppDispatch();
  const timeDispatch = useTimeDispatch();

  const [tenantId, setTenantId] = useState<string | number>(tenantIdState || 0);

  const handleApply = (value: string | number) => {
    const tenantId = Number(value);
    dispatch({ type: "SET_TENANT_ID", payload: tenantId });
    if (serverURL) {
      const updateServerUrl = serverURL.replace(/(\/select\/)([\d]+)(\/prometheus)/gmi, `$1${tenantId}$3`);
      dispatch({ type: "SET_SERVER", payload: updateServerUrl });
      timeDispatch({ type: "RUN_QUERY" });
    }
  };

  const debouncedHandleApply = useCallback(debounce(handleApply, 700), []);

  const handleChange = (value: string) => {
    setTenantId(value);
    debouncedHandleApply(value);
  };

  useEffect(() => {
    if (tenantId === tenantIdState) return;
    setTenantId(tenantIdState);
  }, [tenantIdState]);

  return <TextField
    label="Tenant ID"
    type="number"
    value={tenantId}
    onChange={handleChange}
    endIcon={(
      <Tooltip title={"Define tenant id if you need request to another storage"}>
        <Button
          variant={"text"}
          size={"small"}
          startIcon={<InfoIcon/>}
        />
      </Tooltip>
    )}
  />;
};

export default TenantsConfiguration;
