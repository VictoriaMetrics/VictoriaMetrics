import React, { FC, useState, useMemo } from "preact/compat";
import TextField from "../../../components/Main/TextField/TextField";
import "./style.scss";
import Button from "../../../components/Main/Button/Button";
import classNames from "classnames";
import useDeviceDetect from "../../../hooks/useDeviceDetect";

interface JsonFormProps {
  onUpload: (json: string) => void
  onClose: () => void
}

const JsonForm: FC<JsonFormProps> = ({ onClose, onUpload }) => {
  const { isMobile } = useDeviceDetect();

  const [json, setJson] = useState("");
  const [error, setError] = useState("");

  const errorJson = useMemo(() => {
    try {
      JSON.parse(json);
      return "";
    } catch (e) {
      return e instanceof Error ? e.message : "Unknown error";
    }
  }, [json]);

  const handleChangeJson = (val: string) => {
    setError("");
    setJson(val);
  };

  const handleApply = () => {
    setError(errorJson);
    if (errorJson) return;
    onUpload(json);
    onClose();
  };

  return (
    <div
      className={classNames({
        "vm-json-form vm-json-form_one-field": true,
        "vm-json-form_mobile vm-json-form_one-field_mobile": isMobile,
      })}
    >
      <TextField
        value={json}
        label="JSON"
        type="textarea"
        error={error}
        autofocus
        onChange={handleChangeJson}
        onEnter={handleApply}
      />
      <div className="vm-json-form-footer">
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
