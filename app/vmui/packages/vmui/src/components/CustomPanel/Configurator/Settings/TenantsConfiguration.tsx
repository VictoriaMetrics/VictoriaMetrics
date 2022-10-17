import React, {FC, useState, useEffect, useCallback} from "preact/compat";
import TextField from "@mui/material/TextField";
import InputAdornment from "@mui/material/InputAdornment";
import Tooltip from "@mui/material/Tooltip";
import InfoIcon from "@mui/icons-material/Info";
import {useAppDispatch, useAppState} from "../../../../state/common/StateContext";
import {ChangeEvent} from "react";
import debounce from "lodash.debounce";

const TenantsConfiguration: FC = () => {
  const {tenantId: tenantIdState} = useAppState();
  const dispatch = useAppDispatch();

  const [tenantId, setTenantId] = useState<string | number>(tenantIdState || 0);

  const handleApply = (tenantId: string | number) => dispatch({type: "SET_TENANT_ID", payload: Number(tenantId)});
  const debouncedHandleApply = useCallback(debounce(handleApply, 700), []);

  const handleChange = (e: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    setTenantId(e.target.value);
    debouncedHandleApply(e.target.value);
  };

  useEffect(() => {
    if (tenantId === tenantIdState) return;
    setTenantId(tenantIdState);
  }, [tenantIdState]);

  return <TextField
    label="Tenant ID"
    type="number"
    size="small"
    variant="outlined"
    value={tenantId}
    onChange={handleChange}
    InputProps={{
      startAdornment: (
        <InputAdornment position="start">
          <Tooltip title={"Define tenant id if you need request to another storage"}>
            <InfoIcon fontSize={"small"} />
          </Tooltip>
        </InputAdornment>
      ),
    }}
  />;
};

export default TenantsConfiguration;
