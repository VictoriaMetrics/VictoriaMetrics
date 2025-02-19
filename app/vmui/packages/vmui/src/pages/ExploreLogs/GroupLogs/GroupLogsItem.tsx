import React, { FC, memo, useMemo, useState } from "preact/compat";
import { Logs } from "../../../api/types";
import "./style.scss";
import useBoolean from "../../../hooks/useBoolean";
import { ArrowDownIcon } from "../../../components/Main/Icons";
import classNames from "classnames";
import { useLogsState } from "../../../state/logsPanel/LogsStateContext";
import dayjs from "dayjs";
import { useTimeState } from "../../../state/time/TimeStateContext";
import GroupLogsFieldRow from "./GroupLogsFieldRow";
import { marked } from "marked";
import { useSearchParams } from "react-router-dom";
import { LOGS_DATE_FORMAT, LOGS_URL_PARAMS } from "../../../constants/logs";
import useEventListener from "../../../hooks/useEventListener";
import { getFromStorage } from "../../../utils/storage";

interface Props {
  log: Logs;
  displayFields?: string[];
}

const GroupLogsItem: FC<Props> = ({ log, displayFields = ["_msg"] }) => {
  const {
    value: isOpenFields,
    toggle: toggleOpenFields,
  } = useBoolean(false);

  const [searchParams] = useSearchParams();
  const { markdownParsing } = useLogsState();
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

  const fields = useMemo(() => Object.entries(log), [log]);
  const hasFields = fields.length > 0;

  const displayMessage = useMemo(() => {
    if (displayFields.length) {
      return displayFields.filter(field => log[field]).map((field, i) => (
        <span
          className="vm-group-logs-row-content__sub-msg"
          key={field + i}
        >{log[field]}</span>
      ));
    }
    if (log._msg) return log._msg;
    if (!hasFields) return;
    const dataObject = fields.reduce<{ [key: string]: string }>((obj, [key, value]) => {
      obj[key] = value;
      return obj;
    }, {});
    return JSON.stringify(dataObject);
  }, [log, fields, hasFields, displayFields]);

  const [disabledHovers, setDisabledHovers] = useState(!!getFromStorage("LOGS_DISABLED_HOVERS"));

  const handleUpdateStage = () => {
    const newValDisabledHovers = !!getFromStorage("LOGS_DISABLED_HOVERS");
    if (newValDisabledHovers !== disabledHovers) {
      setDisabledHovers(newValDisabledHovers);
    }
  };

  useEventListener("storage", handleUpdateStage);

  return (
    <div className="vm-group-logs-row">
      <div
        className={classNames({
          "vm-group-logs-row-content": true,
          "vm-group-logs-row-content_interactive": !disabledHovers,
        })}
        onClick={toggleOpenFields}
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
          dangerouslySetInnerHTML={(markdownParsing && formattedMarkdown) ? { __html: formattedMarkdown } : undefined}
        >
          {displayMessage || "-"}
        </div>
      </div>
      {hasFields && isOpenFields && (
        <div
          className={classNames({
            "vm-group-logs-row-fields": true,
            "vm-group-logs-row-fields_interactive": !disabledHovers
          })}
        >
          <table>
            <tbody>
              {fields.map(([key, value]) => (
                <GroupLogsFieldRow
                  key={key}
                  field={key}
                  value={value}
                />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};

export default memo(GroupLogsItem);
