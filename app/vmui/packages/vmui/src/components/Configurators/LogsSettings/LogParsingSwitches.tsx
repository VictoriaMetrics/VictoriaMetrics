import React, { FC } from "preact/compat";
import Switch from "../../Main/Switch/Switch";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import { useLogsDispatch, useLogsState } from "../../../state/logsPanel/LogsStateContext";

const LogParsingSwitches: FC = () => {
  const { isMobile } = useDeviceDetect();
  const { markdownParsing, ansiParsing } = useLogsState();
  const dispatch = useLogsDispatch();

  const handleChangeMarkdownParsing = (val: boolean) => {
    dispatch({ type: "SET_MARKDOWN_PARSING", payload: val });

    if (ansiParsing) {
      dispatch({ type: "SET_ANSI_PARSING", payload: false });
    }
  };

  const handleChangeAnsiParsing = (val: boolean) => {
    dispatch({ type: "SET_ANSI_PARSING", payload: val });

    if (markdownParsing) {
      dispatch({ type: "SET_MARKDOWN_PARSING", payload: false });
    }
  };

  return (
    <>
      <div className="vm-group-logs-configurator-item">
        <Switch
          label={"Enable markdown parsing"}
          value={markdownParsing}
          onChange={handleChangeMarkdownParsing}
          fullWidth={isMobile}
        />
        <div className="vm-group-logs-configurator-item__info">
          Toggle this switch to enable or disable the Markdown formatting for log entries.
          Enabling this will parse log texts to Markdown.
        </div>
      </div>
      <div className="vm-group-logs-configurator-item">
        <Switch
          label={"Enable ANSI parsing"}
          value={ansiParsing}
          onChange={handleChangeAnsiParsing}
          fullWidth={isMobile}
        />
        <div className="vm-group-logs-configurator-item__info">
          Toggle this switch to enable or disable ANSI escape sequence parsing for log entries.
          Enabling this will interpret ANSI codes to render colored log output.
        </div>
      </div>
    </>
  );
};

export default LogParsingSwitches;
