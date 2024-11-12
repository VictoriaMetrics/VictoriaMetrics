import React, { FC, memo, useMemo } from "preact/compat";
import { Logs } from "../../../api/types";
import "./style.scss";
import useBoolean from "../../../hooks/useBoolean";
import { ArrowDownIcon } from "../../../components/Main/Icons";
import classNames from "classnames";
import { useLogsState } from "../../../state/logsPanel/LogsStateContext";
import dayjs from "dayjs";
import { DATE_TIME_FORMAT } from "../../../constants/date";
import { useTimeState } from "../../../state/time/TimeStateContext";
import GroupLogsFieldRow from "./GroupLogsFieldRow";
import { marked } from "marked";

interface Props {
  log: Logs;
}

const GroupLogsItem: FC<Props> = ({ log }) => {
  const {
    value: isOpenFields,
    toggle: toggleOpenFields,
  } = useBoolean(false);

  const { markdownParsing } = useLogsState();
  const { timezone } = useTimeState();

  const formattedTime = useMemo(() => {
    if (!log._time) return "";
    return dayjs(log._time).tz().format(`${DATE_TIME_FORMAT}.SSS`);
  }, [log._time, timezone]);

  const formattedMarkdown = useMemo(() => {
    if (!markdownParsing || !log._msg) return "";
    return marked(log._msg.replace(/```/g, "\n```\n")) as string;
  }, [log._msg, markdownParsing]);

  const fields = useMemo(() => Object.entries(log).filter(([key]) => key !== "_msg"), [log]);
  const hasFields = fields.length > 0;

  const displayMessage = useMemo(() => {
    if (log._msg) return log._msg;
    if (!hasFields) return;
    const dataObject = fields.reduce<{ [key: string]: string }>((obj, [key, value]) => {
      obj[key] = value;
      return obj;
    }, {});
    return JSON.stringify(dataObject);
  }, [log, fields, hasFields]);

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
            "vm-group-logs-row-content__time_missing": !formattedTime
          })}
        >
          {formattedTime || "timestamp missing"}
        </div>
        <div
          className={classNames({
            "vm-group-logs-row-content__msg": true,
            "vm-group-logs-row-content__msg_empty-msg": !log._msg,
            "vm-group-logs-row-content__msg_missing": !displayMessage
          })}
          dangerouslySetInnerHTML={(markdownParsing && formattedMarkdown) ? { __html: formattedMarkdown } : undefined}
        >
          {displayMessage || "-"}
        </div>
      </div>
      {hasFields && isOpenFields && (
        <div className="vm-group-logs-row-fields">
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
