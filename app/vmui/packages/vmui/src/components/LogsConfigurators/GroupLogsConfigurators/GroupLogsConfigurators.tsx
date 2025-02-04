import React, { FC, useMemo, useState } from "preact/compat";
import useBoolean from "../../../hooks/useBoolean";
import { RestartIcon, SettingsIcon } from "../../Main/Icons";
import Button from "../../Main/Button/Button";
import Modal from "../../Main/Modal/Modal";
import Tooltip from "../../Main/Tooltip/Tooltip";
import { Logs } from "../../../api/types";
import Select from "../../Main/Select/Select";
import { useSearchParams } from "react-router-dom";
import "./style.scss";
import Switch from "../../Main/Switch/Switch";
import TextField from "../../Main/TextField/TextField";
import dayjs from "dayjs";
import Hyperlink from "../../Main/Hyperlink/Hyperlink";
import {
  LOGS_DISPLAY_FIELDS,
  LOGS_GROUP_BY,
  LOGS_DATE_FORMAT,
  LOGS_URL_PARAMS,
  WITHOUT_GROUPING
} from "../../../constants/logs";

const {
  GROUP_BY,
  NO_WRAP_LINES,
  COMPACT_GROUP_HEADER,
  DISPLAY_FIELDS,
  DATE_FORMAT
} = LOGS_URL_PARAMS;

const title = "Group view settings";

interface Props {
  logs: Logs[];
}

const GroupLogsConfigurators: FC<Props> = ({ logs }) => {
  const [searchParams, setSearchParams] = useSearchParams();

  const groupBy = searchParams.get(GROUP_BY) || LOGS_GROUP_BY;
  const noWrapLines = searchParams.get(NO_WRAP_LINES) === "true";
  const compactGroupHeader = searchParams.get(COMPACT_GROUP_HEADER) === "true";
  const displayFieldsString = searchParams.get(DISPLAY_FIELDS) || "";
  const displayFields = displayFieldsString ? displayFieldsString.split(",") : [LOGS_DISPLAY_FIELDS];

  const [dateFormat, setDateFormat] = useState(searchParams.get(DATE_FORMAT) || LOGS_DATE_FORMAT);
  const [errorFormat, setErrorFormat] = useState("");

  const isGroupChanged = groupBy !== LOGS_GROUP_BY;
  const isDisplayFieldsChanged = displayFields.length !== 1 || displayFields[0] !== LOGS_DISPLAY_FIELDS;
  const isTimeChanged = searchParams.get(DATE_FORMAT) !== LOGS_DATE_FORMAT;
  const hasChanges = [
    isGroupChanged,
    isDisplayFieldsChanged,
    noWrapLines,
    compactGroupHeader,
    isTimeChanged
  ].some(Boolean);

  const logsKeys = useMemo(() => {
    return Array.from(new Set(logs.map(l => Object.keys(l)).flat()));
  }, [logs]);

  const {
    value: openModal,
    toggle: toggleOpen,
    setFalse: handleClose,
  } = useBoolean(false);

  const handleSelectGroupBy = (key: string) => {
    searchParams.set(GROUP_BY, key);
    setSearchParams(searchParams);
  };

  const handleSelectDisplayField = (value: string) => {
    const prev = displayFields;
    const newDisplayFields = prev.includes(value) ? prev.filter(v => v !== value) : [...prev, value];
    searchParams.set(DISPLAY_FIELDS, newDisplayFields.join(","));
    setSearchParams(searchParams);
  };

  const handleResetDisplayFields = () => {
    searchParams.delete(DISPLAY_FIELDS);
    setSearchParams(searchParams);
  };

  const toggleWrapLines = () => {
    searchParams.set(NO_WRAP_LINES, String(!noWrapLines));
    setSearchParams(searchParams);
  };

  const toggleCompactGroupHeader = () => {
    searchParams.set(COMPACT_GROUP_HEADER, String(!compactGroupHeader));
    setSearchParams(searchParams);
  };

  const handleChangeDateFormat = (format: string) => {
    const date = new Date();
    if (!dayjs(date, format, true).isValid()) {
      setErrorFormat("Invalid date format");
    }
    setDateFormat(format);
  };

  const handleSaveAndClose = () => {
    searchParams.set(DATE_FORMAT, dateFormat);
    setSearchParams(searchParams);
    handleClose();
  };

  const tooltipContent = () => {
    if (!hasChanges) return title;
    return (
      <div className="vm-group-logs-configurator__tooltip">
        <p>{title}</p>
        <hr/>
        <ul>
          {isGroupChanged && <li>Group by <code>{`"${groupBy}"`}</code></li>}
          {isDisplayFieldsChanged && <li>Display fields: {displayFields.length || 1}</li>}
          {noWrapLines && <li>Single-line text is enabled</li>}
          {compactGroupHeader && <li>Compact group header is enabled</li>}
          {isTimeChanged && <li>Date format: <code>{dateFormat}</code></li>}
        </ul>
      </div>
    );
  };

  return (
    <>
      <div className="vm-group-logs-configurator-button">
        <Tooltip title={tooltipContent()}>
          <Button
            variant="text"
            startIcon={<SettingsIcon/>}
            onClick={toggleOpen}
            ariaLabel={title}
          />
        </Tooltip>
        {hasChanges && <span className="vm-group-logs-configurator-button__marker"/>}
      </div>
      {openModal && (
        <Modal
          title={title}
          onClose={handleSaveAndClose}
        >
          <div className="vm-group-logs-configurator">
            <div className="vm-group-logs-configurator-item">
              <Select
                value={groupBy}
                list={[WITHOUT_GROUPING, ...logsKeys]}
                label="Group by field"
                placeholder="Group by field"
                onChange={handleSelectGroupBy}
                searchable
              />
              <Tooltip title={"Reset grouping"}>
                <Button
                  variant="text"
                  color="primary"
                  startIcon={<RestartIcon/>}
                  onClick={() => handleSelectGroupBy(LOGS_GROUP_BY)}
                />
              </Tooltip>
              <span className="vm-group-logs-configurator-item__info">
                Select a field to group logs by (default: <code>{LOGS_GROUP_BY}</code>).
              </span>
            </div>

            <div className="vm-group-logs-configurator-item">
              <Select
                value={displayFields}
                list={logsKeys}
                label="Display fields"
                placeholder="Display fields"
                onChange={handleSelectDisplayField}
                searchable
              />
              <Tooltip title={"Clear fields"}>
                <Button
                  variant="text"
                  color="primary"
                  startIcon={<RestartIcon/>}
                  onClick={handleResetDisplayFields}
                />
              </Tooltip>
              <span className="vm-group-logs-configurator-item__info">
                Select fields to display instead of the message (default: <code>{LOGS_DISPLAY_FIELDS}</code>).
              </span>
            </div>

            <div className="vm-group-logs-configurator-item">
              <TextField
                autofocus
                label="Date format"
                value={dateFormat}
                onChange={handleChangeDateFormat}
                error={errorFormat}
              />
              <Tooltip title={"Reset format"}>
                <Button
                  variant="text"
                  color="primary"
                  startIcon={<RestartIcon/>}
                  onClick={() => setDateFormat(LOGS_DATE_FORMAT)}
                />
              </Tooltip>
              <span className="vm-group-logs-configurator-item__info vm-group-logs-configurator-item__info_input">
                Set the date format (e.g., <code>YYYY-MM-DD HH:mm:ss</code>).
                Learn more in <Hyperlink
                  href="https://day.js.org/docs/en/display/format"
                >this documentation</Hyperlink>. <br/>
                Your current date format: <code>{dayjs().format(dateFormat || LOGS_DATE_FORMAT)}</code>
              </span>
            </div>

            <div className="vm-group-logs-configurator-item">
              <Switch
                value={noWrapLines}
                onChange={toggleWrapLines}
                label="Single-line message"
              />
              <span className="vm-group-logs-configurator-item__info">
                Displays message in a single line and truncates it with an ellipsis if it exceeds the available space
              </span>
            </div>

            <div className="vm-group-logs-configurator-item">
              <Switch
                value={compactGroupHeader}
                onChange={toggleCompactGroupHeader}
                label="Compact group header"
              />
              <span className="vm-group-logs-configurator-item__info">
                Shows group headers in one line with a &quot;+N more&quot; badge for extra fields.
              </span>
            </div>
          </div>
        </Modal>
      )}
    </>
  );
};

export default GroupLogsConfigurators;
