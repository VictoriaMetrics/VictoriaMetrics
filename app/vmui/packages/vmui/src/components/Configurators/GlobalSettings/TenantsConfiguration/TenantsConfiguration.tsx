import React, { FC, useState, useEffect, useRef, useMemo } from "preact/compat";
import { useAppDispatch, useAppState } from "../../../../state/common/StateContext";
import { useTimeDispatch } from "../../../../state/time/TimeStateContext";
import { InfoIcon } from "../../../Main/Icons";
import TextField from "../../../Main/TextField/TextField";
import Button from "../../../Main/Button/Button";
import Tooltip from "../../../Main/Tooltip/Tooltip";
import { useFetchAccountIds } from "./hooks/useFetchAccountIds";
import Autocomplete from "../../../Main/Autocomplete/Autocomplete";
import "./style.scss";
import { replaceTenantId } from "../../../../utils/default-server-url";

const TenantsConfiguration: FC = () => {
  const { tenantId: tenantIdState, serverUrl } = useAppState();
  const dispatch = useAppDispatch();
  const timeDispatch = useTimeDispatch();
  const { accountIds } = useFetchAccountIds();

  const [openAutocomplete, setOpenAutocomplete] = useState(false);
  const autocompleteAnchorEl = useRef<HTMLDivElement>(null);
  const [tenantId, setTenantId] = useState(tenantIdState || "0");

  const autocompleteValue = useMemo(() => {
    return !openAutocomplete ? "" : "(.+)";
  }, [tenantId, openAutocomplete]);

  const handleApply = (value?: string) => {
    const tenant = value || tenantId;
    dispatch({ type: "SET_TENANT_ID", payload: tenant });
    if (serverUrl) {
      const updateServerUrl = replaceTenantId(serverUrl, tenant);
      if (updateServerUrl === serverUrl) return;
      dispatch({ type: "SET_SERVER", payload: updateServerUrl });
      timeDispatch({ type: "RUN_QUERY" });
    }

    if (document.activeElement instanceof HTMLInputElement) {
      document.activeElement.blur();
    }
  };

  const handleChange = (value: string) => {
    setTenantId(value);
  };

  const handleFocus = () => {
    setOpenAutocomplete(true);
  };

  useEffect(() => {
    if (tenantId === tenantIdState) return;
    setTenantId(tenantIdState);
  }, [tenantIdState]);

  useEffect(() => {
    const regexp = /(\/select\/)(\d+|\d.+)(\/)(.+)/;
    const id = (serverUrl.match(regexp) || [])[2];

    if (tenantIdState && tenantIdState !== id) {
      handleApply(tenantIdState);
    } else {
      handleApply(id);
    }
  }, [serverUrl]);

  return (
    <div
      className="vm-tenant-input"
      ref={autocompleteAnchorEl}
    >
      <TextField
        label="Tenant ID"
        value={tenantId}
        onChange={handleChange}
        onEnter={handleApply}
        onFocus={handleFocus}
        onBlur={handleApply}
        endIcon={(
          <Tooltip title="Define tenant id if you need request to another storage">
            <Button
              variant={"text"}
              size={"small"}
              startIcon={<InfoIcon/>}
            />
          </Tooltip>
        )}
      />
      {!!accountIds.length && (
        <Autocomplete
          value={autocompleteValue}
          options={accountIds}
          anchor={autocompleteAnchorEl}
          maxWords={10}
          minLength={0}
          fullWidth
          onSelect={handleApply}
          onOpenAutocomplete={setOpenAutocomplete}
        />
      )}
    </div>
  );
};

export default TenantsConfiguration;
