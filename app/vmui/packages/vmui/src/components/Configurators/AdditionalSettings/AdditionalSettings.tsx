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
import { AUTOCOMPLETE_KEY } from "../../Main/ShortcutKeys/constants/keyList";

const AdditionalSettingsControls: FC<{isMobile?: boolean}> = ({ isMobile }) => {
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

  const handleKeyDown = (e: KeyboardEvent) => {
    const { code, altKey } = e;
    if (code === "KeyA" && altKey) {
      e.preventDefault();
      onChangeAutocomplete();
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
      <Tooltip title={AUTOCOMPLETE_KEY}>
        <Switch
          label={"Autocomplete"}
          value={autocomplete}
          onChange={onChangeAutocomplete}
          fullWidth={isMobile}
        />
      </Tooltip>
      <Switch
        label={"Disable cache"}
        value={nocache}
        onChange={onChangeCache}
        fullWidth={isMobile}
      />
      <Switch
        label={"Trace query"}
        value={isTracingEnabled}
        onChange={onChangeQueryTracing}
        fullWidth={isMobile}
      />
    </div>
  );
};

const AdditionalSettings: FC = () => {
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
          <AdditionalSettingsControls isMobile={isMobile}/>
        </Popper>
      </>
    );
  }

  return <AdditionalSettingsControls/>;
};

export default AdditionalSettings;
