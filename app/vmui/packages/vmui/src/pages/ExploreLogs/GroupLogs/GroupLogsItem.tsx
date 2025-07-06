import React, { FC, memo, useMemo } from "preact/compat";
import { Logs } from "../../../api/types";
import "./style.scss";
import useBoolean from "../../../hooks/useBoolean";
import { ArrowDownIcon, CopyIcon } from "../../../components/Main/Icons";
import classNames from "classnames";
import { useLogsState } from "../../../state/logsPanel/LogsStateContext";
import dayjs from "dayjs";
import { useTimeState } from "../../../state/time/TimeStateContext";
import { marked } from "marked";
import { useSearchParams } from "react-router-dom";
import { LOGS_DATE_FORMAT, LOGS_URL_PARAMS } from "../../../constants/logs";
import { parseAnsiToHtml } from "../../../utils/ansiParser";
import GroupLogsFields from "./GroupLogsFields";
import { useLocalStorageBoolean } from "../../../hooks/useLocalStorageBoolean";
import Button from "../../../components/Main/Button/Button";
import Tooltip from "../../../components/Main/Tooltip/Tooltip";
import { useCallback, useEffect, useState } from "react";
import useCopyToClipboard from "../../../hooks/useCopyToClipboard";

interface Props {
  log: Logs;
  displayFields?: string[];
  hideGroupButton?: boolean;
  onItemClick?: (log: Logs) => void;
}

const GroupLogsItem: FC<Props> = ({ log, displayFields = ["_msg"], onItemClick, hideGroupButton }) => {
  const {
    value: isOpenFields,
    toggle: toggleOpenFields,
  } = useBoolean(false);
  const [copied, setCopied] = useState<boolean>(false);
  const copyToClipboard = useCopyToClipboard();

  const [searchParams] = useSearchParams();
  const { markdownParsing, ansiParsing } = useLogsState();
  const { timezone } = useTimeState();

  const noWrapLines = searchParams.get(LOGS_URL_PARAMS.NO_WRAP_LINES) === "true";
  const dateFormat = searchParams.get(LOGS_URL_PARAMS.DATE_FORMAT) || LOGS_DATE_FORMAT;

  const formattedTime = useMemo(() => {
    if (!log._time) return "";
    return dayjs(log._time).tz().format(dateFormat);
  }, [log._time, timezone, dateFormat]);

  const formattedMarkdown = useMemo(() => {
    if (!markdownParsing || !log._msg) return "";
    return marked(log._msg.replace(/```/g, "\n```\n")) as string;
  }, [log._msg, markdownParsing]);

  const hasFields = Object.keys(log).length > 0;

  const displayMessage = useMemo(() => {
    const values: (string | React.ReactNode)[] = [];

    if (!hasFields) {
      values.push("-");
    }

    if (displayFields.some(field => log[field])) {
      displayFields.filter(field => log[field]).forEach((field) => {
        const value = field === "_msg" && ansiParsing ? parseAnsiToHtml(log[field]) : log[field];
        values.push(value);
      });
    } else {
      Object.entries(log).forEach(([key, value]) => {
        values.push(`${key}: ${value}`);
      });
    }

    return values;
  }, [log, hasFields, displayFields, ansiParsing]);

  const [disabledHovers] = useLocalStorageBoolean("LOGS_DISABLED_HOVERS");

  const handleClick = () => {
    toggleOpenFields();
    onItemClick?.(log);
  };

  const handleCopy = useCallback(async (e: Event) => {
    e.stopPropagation();
    if (copied) return;
    try {
      await copyToClipboard(JSON.stringify(log, null, 2));
      setCopied(true);
    } catch (e) {
      console.error(e);
    }
  }, [copied, copyToClipboard]);

  useEffect(() => {
    if (copied === null) return;
    const timeout = setTimeout(() => setCopied(false), 2000);
    return () => clearTimeout(timeout);
  }, [copied]);

  return (
    <div className="vm-group-logs-row">
      <div
        className={classNames({
          "vm-group-logs-row-content": true,
          "vm-group-logs-row-content_interactive": !disabledHovers,
        })}
        onClick={handleClick}
      >
        <Tooltip title={copied ? "Copied" : "Copy to clipboard"}>
          <Button
            className="vm-group-logs-row-content__copy-row"
            variant="text"
            color="gray"
            size="small"
            startIcon={<CopyIcon/>}
            onClick={handleCopy}
            ariaLabel="copy to clipboard"
          />
        </Tooltip>
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
            "vm-group-logs-row-content__time_missing": !formattedTime
          })}
        >
          {formattedTime || "timestamp missing"}
        </div>
        <div
          className={classNames({
            "vm-group-logs-row-content__msg": true,
            "vm-group-logs-row-content__msg_empty-msg": !log._msg,
            "vm-group-logs-row-content__msg_missing": !displayMessage,
            "vm-group-logs-row-content__msg_single-line": noWrapLines,
          })}
          dangerouslySetInnerHTML={formattedMarkdown ? { __html: formattedMarkdown } : undefined}
        >
          {displayMessage.map((msg, i) => (
            <span
              className="vm-group-logs-row-content__sub-msg"
              key={`${msg}_${i}`}
            >
              {msg}
            </span>
          ))}
        </div>
      </div>
      {hasFields && isOpenFields && <GroupLogsFields
        hideGroupButton={hideGroupButton}
        log={log}
      />}
    </div>
  );
};

export default memo(GroupLogsItem);
