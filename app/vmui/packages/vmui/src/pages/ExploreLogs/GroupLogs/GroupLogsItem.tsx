import React, { FC, useEffect, useMemo, useState } from "preact/compat";
import { Logs } from "../../../api/types";
import "./style.scss";
import useBoolean from "../../../hooks/useBoolean";
import Button from "../../../components/Main/Button/Button";
import Tooltip from "../../../components/Main/Tooltip/Tooltip";
import { ArrowDownIcon, CopyIcon } from "../../../components/Main/Icons";
import useCopyToClipboard from "../../../hooks/useCopyToClipboard";
import classNames from "classnames";

interface Props {
  log: Logs;
  markdownParsing: boolean;
}

const GroupLogsItem: FC<Props> = ({ log, markdownParsing }) => {
  const {
    value: isOpenFields,
    toggle: toggleOpenFields,
  } = useBoolean(false);

  const excludeKeys = ["_stream", "_msg", "_time", "_vmui_time", "_vmui_data", "_vmui_markdown"];
  const fields = Object.entries(log).filter(([key]) => !excludeKeys.includes(key));
  const hasFields = fields.length > 0;

  const displayMessage = useMemo(() => {
    if (log._msg) return log._msg;
    if (!hasFields) return;
    const dataObject = fields.reduce<{[key: string]: string}>((obj, [key, value]) => {
      obj[key] = value;
      return obj;
    }, {});
    return JSON.stringify(dataObject);
  }, [log, fields, hasFields]);

  const copyToClipboard = useCopyToClipboard();
  const [copied, setCopied] = useState<number | null>(null);

  const createCopyHandler = (copyValue:  string, rowIndex: number) => async () => {
    if (copied === rowIndex) return;
    try {
      await copyToClipboard(copyValue);
      setCopied(rowIndex);
    } catch (e) {
      console.error(e);
    }
  };

  useEffect(() => {
    if (copied === null) return;
    const timeout = setTimeout(() => setCopied(null), 2000);
    return () => clearTimeout(timeout);
  }, [copied]);

  return (
    <div className="vm-group-logs-row">
      <div
        className="vm-group-logs-row-content"
        onClick={toggleOpenFields}
        key={`${log._msg}${log._time}`}
      >
        {hasFields && (
          <div
            className={classNames({
              "vm-group-logs-row-content__arrow": true,
              "vm-group-logs-row-content__arrow_open": isOpenFields,
            })}
          >
            <ArrowDownIcon/>
          </div>
        )}
        <div
          className={classNames({
            "vm-group-logs-row-content__time": true,
            "vm-group-logs-row-content__time_missing": !log._vmui_time
          })}
        >
          {log._vmui_time || "timestamp missing"}
        </div>
        <div
          className={classNames({
            "vm-group-logs-row-content__msg": true,
            "vm-group-logs-row-content__msg_empty-msg": !log._msg,
            "vm-group-logs-row-content__msg_missing": !displayMessage
          })}
          dangerouslySetInnerHTML={markdownParsing && log._vmui_markdown ? { __html: log._vmui_markdown } : undefined}
        >
          {displayMessage || "-"}
        </div>
      </div>
      {hasFields && isOpenFields && (
        <div className="vm-group-logs-row-fields">
          <table>
            <tbody>
              {fields.map(([key, value], i) => (
                <tr
                  key={key}
                  className="vm-group-logs-row-fields-item"
                >
                  <td className="vm-group-logs-row-fields-item-controls">
                    <div className="vm-group-logs-row-fields-item-controls__wrapper">
                      <Tooltip title={copied === i ? "Copied" : "Copy to clipboard"}>
                        <Button
                          variant="text"
                          color="gray"
                          size="small"
                          startIcon={<CopyIcon/>}
                          onClick={createCopyHandler(`${key}: "${value}"`, i)}
                          ariaLabel="copy to clipboard"
                        />
                      </Tooltip>
                    </div>
                  </td>
                  <td className="vm-group-logs-row-fields-item__key">{key}</td>
                  <td className="vm-group-logs-row-fields-item__value">{value}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};

export default GroupLogsItem;
