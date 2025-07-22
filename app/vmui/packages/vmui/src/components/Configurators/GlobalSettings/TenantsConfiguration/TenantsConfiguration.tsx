import { FC, useState, useRef, useEffect, useMemo } from "preact/compat";
import { useAppDispatch, useAppState } from "../../../../state/common/StateContext";
import { useTimeDispatch } from "../../../../state/time/TimeStateContext";
import { ArrowDownIcon, StorageIcon } from "../../../Main/Icons";
import Button from "../../../Main/Button/Button";
import "./style.scss";
import classNames from "classnames";
import Popper from "../../../Main/Popper/Popper";
import { getAppModeEnable } from "../../../../utils/app-mode";
import Tooltip from "../../../Main/Tooltip/Tooltip";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";
import TextField from "../../../Main/TextField/TextField";
import { getTenantIdFromUrl, replaceTenantId } from "../../../../utils/tenants";
import useBoolean from "../../../../hooks/useBoolean";

const TenantsConfiguration: FC<{accountIds: string[]}> = ({ accountIds }) => {
  const appModeEnable = getAppModeEnable();
  const { isMobile } = useDeviceDetect();

  const { tenantId: tenantIdState, serverUrl } = useAppState();
  const dispatch = useAppDispatch();
  const timeDispatch = useTimeDispatch();

  const [search, setSearch] = useState("");
  const optionsButtonRef = useRef<HTMLDivElement>(null);

  const {
    value: openOptions,
    toggle: toggleOpenOptions,
    setFalse: handleCloseOptions,
  } = useBoolean(false);

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

  const showTenantSelector = useMemo(() => accountIds.length > 1, [accountIds]);

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
          {isMobile ? (
            <div
              className="vm-mobile-option"
              onClick={toggleOpenOptions}
            >
              <span className="vm-mobile-option__icon"><StorageIcon/></span>
              <div className="vm-mobile-option-text">
                <span className="vm-mobile-option-text__label">Tenant ID</span>
                <span className="vm-mobile-option-text__value">{tenantIdState}</span>
              </div>
              <span className="vm-mobile-option__arrow"><ArrowDownIcon/></span>
            </div>
          ) : (
            <Button
              className={appModeEnable ? "" : "vm-header-button"}
              variant="contained"
              color="primary"
              fullWidth
              startIcon={<StorageIcon/>}
              endIcon={(
                <div
                  className={classNames({
                    "vm-execution-controls-buttons__arrow": true,
                    "vm-execution-controls-buttons__arrow_open": openOptions,
                  })}
                >
                  <ArrowDownIcon/>
                </div>
              )}
              onClick={toggleOpenOptions}
            >
              {tenantIdState}
            </Button>
          )}
        </div>
      </Tooltip>
      <Popper
        open={openOptions}
        placement="bottom-right"
        onClose={handleCloseOptions}
        buttonRef={optionsButtonRef}
        title={isMobile ? "Define Tenant ID" : undefined}
      >
        <div
          className={classNames({
            "vm-list vm-tenant-input-list": true,
            "vm-list vm-tenant-input-list_mobile": isMobile,
          })}
        >
          <div className="vm-tenant-input-list__search">
            <TextField
              autofocus
              label="Search"
              value={search}
              onChange={setSearch}
              type="search"
            />
          </div>
          {accountIdsFiltered.map(id => (
            <div
              className={classNames({
                "vm-list-item": true,
                "vm-list-item_mobile": isMobile,
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
