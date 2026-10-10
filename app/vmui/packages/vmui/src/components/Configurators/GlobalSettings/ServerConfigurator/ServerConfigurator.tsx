import { forwardRef, useCallback, useEffect, useImperativeHandle, useState } from "preact/compat";
import { ErrorTypes } from "../../../../types";
import TextField from "../../../Main/TextField/TextField";
import { isValidHttpUrl } from "../../../../utils/url";
import Button from "../../../Main/Button/Button";
import { UnlockIcon } from "../../../Main/Icons";
import { getFromStorage, removeFromStorage, saveToStorage } from "../../../../utils/storage";
import useBoolean from "../../../../hooks/useBoolean";
import { ChildComponentHandle } from "../GlobalSettings";
import { useAppDispatch, useAppState } from "../../../../state/common/StateContext";
import Checkbox from "../../../Main/Checkbox/Checkbox";
import Hyperlink from "../../../Main/Hyperlink/Hyperlink";
import Alert from "../../../Main/Alert/Alert";

interface ServerConfiguratorProps {
  onClose: () => void;
}

const ServerConfigurator = forwardRef<ChildComponentHandle, ServerConfiguratorProps>(({ onClose }, ref) => {
  const { serverUrl: stateServerUrl } = useAppState();
  const dispatch = useAppDispatch();

  const {
    value: editable,
    setTrue: setEditable,
  } = useBoolean(false);

  const {
    value: enabledStorage,
    toggle: handleToggleStorage,
  } = useBoolean(!!getFromStorage("SERVER_URL"));

  const [serverUrl, setServerUrl] = useState(stateServerUrl);
  const [error, setError] = useState("");

  const handleChange = (val: string) => {
    const value = val || "";
    setServerUrl(value);
    setError("");
  };

  const handleApply = useCallback(() => {
    dispatch({ type: "SET_SERVER", payload: serverUrl });
    onClose();
  }, [serverUrl]);

  useEffect(() => {
    if (!stateServerUrl) setError(ErrorTypes.emptyServer);
    if (!isValidHttpUrl(stateServerUrl)) setError(ErrorTypes.validServer);
  }, [stateServerUrl]);

  useEffect(() => {
    if (enabledStorage) {
      saveToStorage("SERVER_URL", serverUrl);
    } else {
      removeFromStorage(["SERVER_URL"]);
    }
  }, [enabledStorage]);

  useEffect(() => {
    // the tenant selector can change the serverUrl
    if (stateServerUrl === serverUrl) return;
    setServerUrl(stateServerUrl);
  }, [stateServerUrl]);

  useImperativeHandle(ref, () => ({ handleApply }), [handleApply]);

  return (
    <div>
      <div className="vm-server-configurator__title">
        Server URL
      </div>
      <div className="vm-server-configurator-url">
        <TextField
          autofocus
          value={serverUrl}
          error={error}
          onChange={handleChange}
          onEnter={handleApply}
          inputmode="url"
          disabled={!editable}
        />
        {!editable && (
          <Button
            className="vm-server-configurator-url__button"
            variant="text"
            color="primary"
            onClick={setEditable}
            startIcon={<UnlockIcon/>}
          >
            Enable editing
          </Button>
        )}
      </div>
      <Checkbox
        label="Keep this URL after page refresh"
        color="primary"
        checked={enabledStorage}
        onChange={handleToggleStorage}
      />

      <div className="vm-server-configurator-url__warning">
        <Alert
          variant="warning"
          title="Server URL editing will be removed in a future release."
        >
          <p>
            Need a custom URL? <Hyperlink
              href="https://github.com/VictoriaMetrics/VictoriaMetrics/issues/11735"
              text="Share your use case in #11735."
            />
          </p>
        </Alert>
      </div>
    </div>
  );
});

export default ServerConfigurator;
