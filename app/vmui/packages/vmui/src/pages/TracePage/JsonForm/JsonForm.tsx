import React, { FC, useState, useMemo } from "preact/compat";
import TextField from "../../../components/Main/TextField/TextField";
import "./style.scss";
import Button from "../../../components/Main/Button/Button";
import Trace from "../../../components/TraceQuery/Trace";
import { ErrorTypes } from "../../../types";
import classNames from "classnames";
import useCopyToClipboard from "../../../hooks/useCopyToClipboard";
import { CopyIcon, RestartIcon } from "../../../components/Main/Icons";
import useDeviceDetect from "../../../hooks/useDeviceDetect";

interface JsonFormProps {
  defaultJson?: string
  defaultTile?: string
  displayTitle?: boolean
  editable?: boolean
  resetValue?: string
  onUpload: (json: string, title: string) => void
  onClose: () => void
}

const JsonForm: FC<JsonFormProps> = ({
  editable = false,
  defaultTile = "JSON",
  displayTitle = true,
  defaultJson = "",
  resetValue= "",
  onClose,
  onUpload,
}) => {
  const copyToClipboard = useCopyToClipboard();
  const { isMobile } = useDeviceDetect();

  const [json, setJson] = useState(defaultJson);
  const [title, setTitle] = useState(defaultTile);
  const [errorTitle, setErrorTitle] = useState("");
  const [error, setError] = useState("");

  const errorJson = useMemo(() => {
    try {
      const resp = JSON.parse(json);
      const traceData = resp.trace || resp;
      if (!traceData.duration_msec) return ErrorTypes.traceNotFound;
      new Trace(traceData, "");
      return "";
    } catch (e) {
      return e instanceof Error ? e.message : "Unknown error";
    }
  }, [json]);

  const handleChangeTitle = (val: string) => {
    setTitle(val);
  };

  const handleChangeJson = (val: string) => {
    setError("");
    setJson(val);
  };

  const handlerCopy = async () => {
    await copyToClipboard(json, "Formatted JSON has been copied");
  };

  const handleReset = () => {
    setJson(resetValue);
  };

  const handleApply = () => {
    setError(errorJson);
    const titleTrim = title.trim();
    if (!titleTrim) setErrorTitle(ErrorTypes.emptyTitle);
    if (errorJson || errorTitle) return;
    onUpload(json, title);
    onClose();
  };

  return (
    <div
      className={classNames({
        "vm-json-form": true,
        "vm-json-form_one-field": !displayTitle,
        "vm-json-form_one-field_mobile": !displayTitle && isMobile,
        "vm-json-form_mobile": isMobile
      })}
    >
      {displayTitle && (
        <TextField
          value={title}
          label="Title"
          error={errorTitle}
          onEnter={handleApply}
          onChange={handleChangeTitle}
        />
      )}
      <TextField
        value={json}
        label="JSON"
        type="textarea"
        error={error}
        autofocus
        onChange={handleChangeJson}
        onEnter={handleApply}
        disabled={!editable}
      />
      <div className="vm-json-form-footer">
        <div className="vm-json-form-footer__controls">
          <Button
            variant="outlined"
            startIcon={<CopyIcon/>}
            onClick={handlerCopy}
          >
            Copy JSON
          </Button>
          {resetValue && (
            <Button
              variant="text"
              startIcon={<RestartIcon/>}
              onClick={handleReset}
            >
              Reset JSON
            </Button>
          )}
        </div>
        <div className="vm-json-form-footer__controls vm-json-form-footer__controls_right">
          <Button
            variant="outlined"
            color="error"
            onClick={onClose}
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={handleApply}
          >
            apply
          </Button>
        </div>
      </div>
    </div>
  );
};

export default JsonForm;
