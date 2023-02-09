import React, { FC, useState, useRef, useEffect, useMemo } from "preact/compat";
import { useAppDispatch, useAppState } from "../../../../state/common/StateContext";
import { useTimeDispatch } from "../../../../state/time/TimeStateContext";
import { ArrowDownIcon, StorageIcon } from "../../../Main/Icons";
import Button from "../../../Main/Button/Button";
import "./style.scss";
import { replaceTenantId } from "../../../../utils/default-server-url";
import classNames from "classnames";
import Popper from "../../../Main/Popper/Popper";
import { getAppModeEnable } from "../../../../utils/app-mode";
import Tooltip from "../../../Main/Tooltip/Tooltip";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";
import TextField from "../../../Main/TextField/TextField";

const TenantsConfiguration: FC<{accountIds: string[]}> = ({ accountIds }) => {
  const appModeEnable = getAppModeEnable();
  const { isMobile } = useDeviceDetect();

  const { tenantId: tenantIdState, serverUrl } = useAppState();
  const dispatch = useAppDispatch();
  const timeDispatch = useTimeDispatch();

  const [search, setSearch] = useState("");
  const [openOptions, setOpenOptions] = useState(false);
  const optionsButtonRef = useRef<HTMLDivElement>(null);

  const accountIdsFiltered = useMemo(() => {
    if (!search) return accountIds;
    try {
      const regexp = new RegExp(search, "i");
      const found = accountIds.filter((item) => regexp.test(item));
      return found.sort((a,b) => (a.match(regexp)?.index || 0) - (b.match(regexp)?.index || 0));
    } catch (e) {
      return [];
    }
  }, [search, accountIds]);

  const getTenantIdFromUrl = (url: string) => {
    const regexp = /(\/select\/)(\d+|\d.+)(\/)(.+)/;
    return (url.match(regexp) || [])[2];
  };

  const showTenantSelector = useMemo(() => {
    const id = getTenantIdFromUrl(serverUrl);
    return accountIds.length > 1 && id;
  }, [accountIds, serverUrl]);

  const toggleOpenOptions = () => {
    setOpenOptions(prev => !prev);
  };

  const handleCloseOptions = () => {
    setOpenOptions(false);
  };

  const createHandlerChange = (value: string) => () => {
    const tenant = value;
    dispatch({ type: "SET_TENANT_ID", payload: tenant });
    if (serverUrl) {
      const updateServerUrl = replaceTenantId(serverUrl, tenant);
      if (updateServerUrl === serverUrl) return;
      dispatch({ type: "SET_SERVER", payload: updateServerUrl });
      timeDispatch({ type: "RUN_QUERY" });
    }
    handleCloseOptions();
  };

  useEffect(() => {
    const id = getTenantIdFromUrl(serverUrl);

    if (tenantIdState && tenantIdState !== id) {
      createHandlerChange(tenantIdState)();
    } else {
      createHandlerChange(id)();
    }
  }, [serverUrl]);

  if (!showTenantSelector) return null;

  return (
    <div className="vm-tenant-input">
      <Tooltip title="Define Tenant ID if you need request to another storage">
        <div ref={optionsButtonRef}>
          <Button
            className={appModeEnable ? "" : "vm-header-button"}
            variant="contained"
            color="primary"
            fullWidth
            startIcon={<StorageIcon/>}
            endIcon={!isMobile ? (
              <div
                className={classNames({
                  "vm-execution-controls-buttons__arrow": true,
                  "vm-execution-controls-buttons__arrow_open": openOptions,
                })}
              >
                <ArrowDownIcon/>
              </div>
            ) : undefined}
            onClick={toggleOpenOptions}
          >
            {!isMobile && tenantIdState}
          </Button>
        </div>
      </Tooltip>
      <Popper
        open={openOptions}
        placement="bottom-right"
        onClose={handleCloseOptions}
        buttonRef={optionsButtonRef}
      >
        <div className="vm-list vm-tenant-input-list">
          <div className="vm-tenant-input-list__search">
            <TextField
              autofocus
              label="Search"
              value={search}
              onChange={setSearch}
            />
          </div>
          {accountIdsFiltered.map(id => (
            <div
              className={classNames({
                "vm-list-item": true,
                "vm-list-item_active": id === tenantIdState
              })}
              key={id}
              onClick={createHandlerChange(id)}
            >
              {id}
            </div>
          ))}
        </div>
      </Popper>
    </div>
  );
};

export default TenantsConfiguration;
