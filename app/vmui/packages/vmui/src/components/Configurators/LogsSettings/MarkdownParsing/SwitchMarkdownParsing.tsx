import React, { FC } from "preact/compat";
import Switch from "../../../Main/Switch/Switch";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";
import { useLogsDispatch, useLogsState } from "../../../../state/logsPanel/LogsStateContext";

const SwitchMarkdownParsing: FC = () => {
  const { isMobile } = useDeviceDetect();
  const { markdownParsing } = useLogsState();
  const dispatch = useLogsDispatch();


  const handleChangeMarkdownParsing = (val: boolean) => {
    dispatch({ type: "SET_MARKDOWN_PARSING", payload: val });
  };

  return (
    <div>
      <div className="vm-server-configurator__title">
        Markdown Parsing for Logs
      </div>
      <Switch
        label={markdownParsing ? "Disable markdown parsing" : "Enable markdown parsing"}
        value={markdownParsing}
        onChange={handleChangeMarkdownParsing}
        fullWidth={isMobile}
      />
      <div className="vm-server-configurator__info">
        Toggle this switch to enable or disable the Markdown formatting for log entries.
        Enabling this will parse log texts to Markdown.
      </div>
    </div>
  );
};

export default SwitchMarkdownParsing;
