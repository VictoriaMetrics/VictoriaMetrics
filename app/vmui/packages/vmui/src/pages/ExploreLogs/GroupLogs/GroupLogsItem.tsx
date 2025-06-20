import { FC, memo, ReactNode, useMemo, MouseEvent } from "react";
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

const GroupLogsItem: FC<Props> = ({ log, displayFields = [], onItemClick, hideGroupButton }) => {
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
    if (!markdownParsing || !log._msg || !displayFields.includes("_msg")) return "";
    return marked(log._msg.replace(/```/g, "\n```\n")) as string;
  }, [log._msg, markdownParsing, displayFields]);

  const hasFields = Object.keys(log).length > 0;

  const displayMessage = useMemo(() => {
    const values: (string | ReactNode)[] = [];

    if (!hasFields) {
      values.push("-");
    }

    if (displayFields.some(field => log[field])) {
      displayFields.filter(field => log[field]).forEach((field) => {
        let value: string | ReactNode[] = log[field];

        const isMessageField = field === "_msg";

        if (isMessageField && ansiParsing) {
          value = parseAnsiToHtml(log[field]);
        }

        if (isMessageField && markdownParsing) {
          value = "";
        }

        value && values.push(value);
      });
    } else {
      Object.entries(log).forEach(([key, value]) => {
        values.push(`${key}: ${value}`);
      });
    }

    return values;
  }, [log, hasFields, displayFields, ansiParsing, markdownParsing]);

  const [disabledHovers] = useLocalStorageBoolean("LOGS_DISABLED_HOVERS");

  const handleClick = () => {
    toggleOpenFields();
    onItemClick?.(log);
  };

  const handleCopy = useCallback(async (e: MouseEvent) => {
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
        >
          {formattedMarkdown && <div dangerouslySetInnerHTML={{ __html: formattedMarkdown }}/>}
          {displayMessage.map((msg, i) => (
            <p
              className="vm-group-logs-row-content__sub-msg"
              key={`${msg}_${i}`}
            >
              {msg}
            </p>
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
