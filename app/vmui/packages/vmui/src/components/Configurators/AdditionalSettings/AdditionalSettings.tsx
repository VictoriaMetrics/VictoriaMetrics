import React, { FC, useRef } from "preact/compat";
import { useCustomPanelDispatch, useCustomPanelState } from "../../../state/customPanel/CustomPanelStateContext";
import { useQueryDispatch, useQueryState } from "../../../state/query/QueryStateContext";
import "./style.scss";
import Switch from "../../Main/Switch/Switch";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import Popper from "../../Main/Popper/Popper";
import { TuneIcon } from "../../Main/Icons";
import Button from "../../Main/Button/Button";
import classNames from "classnames";
import useBoolean from "../../../hooks/useBoolean";
import useEventListener from "../../../hooks/useEventListener";
import Tooltip from "../../Main/Tooltip/Tooltip";
import { AUTOCOMPLETE_QUICK_KEY } from "../../Main/ShortcutKeys/constants/keyList";
import { QueryConfiguratorProps } from "../../../pages/CustomPanel/QueryConfigurator/QueryConfigurator";

type Props = Pick<QueryConfiguratorProps, "hideButtons">;

const AdditionalSettingsControls: FC<Props & {isMobile?: boolean}> = ({ isMobile, hideButtons }) => {
  const { autocomplete } = useQueryState();
  const queryDispatch = useQueryDispatch();

  const { nocache, isTracingEnabled, reduceMemUsage } = useCustomPanelState();
  const customPanelDispatch = useCustomPanelDispatch();

  const onChangeCache = () => {
    customPanelDispatch({ type: "TOGGLE_NO_CACHE" });
  };

  const onChangeReduceMemUsage = () => {
    customPanelDispatch({ type: "TOGGLE_REDUCE_MEM_USAGE" });
  };

  const onChangeQueryTracing = () => {
    customPanelDispatch({ type: "TOGGLE_QUERY_TRACING" });
  };

  const onChangeAutocomplete = () => {
    queryDispatch({ type: "TOGGLE_AUTOCOMPLETE" });
  };

  const onChangeQuickAutocomplete = () => {
    queryDispatch({ type: "SET_AUTOCOMPLETE_QUICK", payload: true });
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    /** @see AUTOCOMPLETE_QUICK_KEY */
    const { code, ctrlKey, altKey } = e;
    if (code === "Space" && (ctrlKey || altKey)) {
      e.preventDefault();
      onChangeQuickAutocomplete();
    }
  };

  useEventListener("keydown", handleKeyDown);

  return (
    <div
      className={classNames({
        "vm-additional-settings": true,
        "vm-additional-settings_mobile": isMobile
      })}
    >
      {!hideButtons?.autocomplete && (
        <Tooltip title={<>Quick tip: {AUTOCOMPLETE_QUICK_KEY}</>}>
          <Switch
            label={"Autocomplete"}
            value={autocomplete}
            onChange={onChangeAutocomplete}
            fullWidth={isMobile}
          />
        </Tooltip>
      )}
      {!hideButtons?.disableCache && (
        <Switch
          label={"Disable cache"}
          value={nocache}
          onChange={onChangeCache}
          fullWidth={isMobile}
        />
      )}
      {!hideButtons?.reduceMemUsage && (
        <Switch
          label={"Disable deduplication"}
          value={reduceMemUsage}
          onChange={onChangeReduceMemUsage}
          fullWidth={isMobile}
        />
      )}
      {!hideButtons?.traceQuery && (
        <Switch
          label={"Trace query"}
          value={isTracingEnabled}
          onChange={onChangeQueryTracing}
          fullWidth={isMobile}
        />
      )}
    </div>
  );
};

const AdditionalSettings: FC<Props> = (props) => {
  const { isMobile } = useDeviceDetect();
  const targetRef = useRef<HTMLDivElement>(null);

  const {
    value: openList,
    toggle: handleToggleList,
    setFalse: handleCloseList,
  } = useBoolean(false);

  if (isMobile) {
    return (
      <>
        <div ref={targetRef}>
          <Button
            variant="outlined"
            startIcon={<TuneIcon/>}
            onClick={handleToggleList}
            ariaLabel="additional the query settings"
          />
        </div>
        <Popper
          open={openList}
          buttonRef={targetRef}
          placement="bottom-left"
          onClose={handleCloseList}
          title={"Query settings"}
        >
          <AdditionalSettingsControls
            isMobile={isMobile}
            {...props}
          />
        </Popper>
      </>
    );
  }

  return <AdditionalSettingsControls {...props}/>;
};

export default AdditionalSettings;
