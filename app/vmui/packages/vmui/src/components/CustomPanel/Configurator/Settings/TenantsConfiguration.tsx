import React, {FC, useState, useEffect} from "preact/compat";
import Box from "@mui/material/Box";
import TextField from "@mui/material/TextField";
import InputAdornment from "@mui/material/InputAdornment";
import Tooltip from "@mui/material/Tooltip";
import InfoIcon from "@mui/icons-material/Info";
import Button from "@mui/material/Button";
import {useAppDispatch, useAppState} from "../../../../state/common/StateContext";
import {ChangeEvent} from "react";


const TenantsConfiguration: FC = () => {
  const {tenantId: tenantIdState} = useAppState();
  const dispatch = useAppDispatch();

  const [tenantId, setTenantId] = useState<string | number>(tenantIdState || 0);

  const handleChange = (e: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    setTenantId(e.target.value);
  };

  const handleApply = () => {
    dispatch({type: "SET_TENANT_ID", payload: Number(tenantId)});
  };

  useEffect(() => {
    if (tenantId === tenantIdState) return;
    setTenantId(tenantIdState);
  }, [tenantIdState]);

  return <Box display={"flex"} alignItems={"stretch"}>
    <TextField
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
    />
    <Button
      variant={"outlined"}
      onClick={handleApply}
      color="primary"
      sx={{ml: 1, minHeight: "100%"}}
    >
      Apply
    </Button>
  </Box>;
};

export default TenantsConfiguration;
