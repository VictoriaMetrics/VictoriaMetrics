import React, { FC, useCallback, useEffect, useMemo, useRef } from "preact/compat";
import { MouseEvent, useState } from "react";
import "./style.scss";
import { Logs } from "../../../api/types";
import Accordion from "../../../components/Main/Accordion/Accordion";
import { groupByMultipleKeys } from "../../../utils/array";
import Tooltip from "../../../components/Main/Tooltip/Tooltip";
import useCopyToClipboard from "../../../hooks/useCopyToClipboard";
import GroupLogsItem from "./GroupLogsItem";
import { useAppState } from "../../../state/common/StateContext";
import classNames from "classnames";
import Button from "../../../components/Main/Button/Button";
import { CollapseIcon, ExpandIcon, StorageIcon } from "../../../components/Main/Icons";
import Popper from "../../../components/Main/Popper/Popper";
import TextField from "../../../components/Main/TextField/TextField";
import useBoolean from "../../../hooks/useBoolean";
import useStateSearchParams from "../../../hooks/useStateSearchParams";
import { useSearchParams } from "react-router-dom";
import { getStreamPairs } from "../../../utils/logs";

const WITHOUT_GROUPING = "No Grouping";

interface TableLogsProps {
  logs: Logs[];
  columns: string[];
  settingsRef: React.RefObject<HTMLElement>;
}

const GroupLogs: FC<TableLogsProps> = ({ logs, settingsRef }) => {
  const { isDarkTheme } = useAppState();
  const copyToClipboard = useCopyToClipboard();
  const [searchParams, setSearchParams] = useSearchParams();

  const [expandGroups, setExpandGroups] = useState<boolean[]>([]);
  const [groupBy, setGroupBy] = useStateSearchParams("_stream", "groupBy");
  const [copied, setCopied] = useState<string | null>(null);
  const [searchKey, setSearchKey] = useState("");
  const optionsButtonRef = useRef<HTMLDivElement>(null);

  const {
    value: openOptions,
    toggle: toggleOpenOptions,
    setFalse: handleCloseOptions,
  } = useBoolean(false);

  const expandAll = useMemo(() => expandGroups.every(Boolean), [expandGroups]);

  const logsKeys = useMemo(() => {
    const excludeKeys = ["_msg", "_time", "_vmui_time", "_vmui_data", "_vmui_markdown"];
    const uniqKeys = Array.from(new Set(logs.map(l => Object.keys(l)).flat()));
    const keys = [WITHOUT_GROUPING, ...uniqKeys.filter(k => !excludeKeys.includes(k))];

    if (!searchKey) return keys;
    try {
      const regexp = new RegExp(searchKey, "i");
      const found = keys.filter((item) => regexp.test(item));
      return found.sort((a,b) => (a.match(regexp)?.index || 0) - (b.match(regexp)?.index || 0));
    } catch (e) {
      return [];
    }
  }, [logs, searchKey]);

  const groupData = useMemo(() => {
    return groupByMultipleKeys(logs, [groupBy]).map((item) => {
      const streamValue = item.values[0]?.[groupBy] || "";
      const pairs = getStreamPairs(streamValue);
      return {
        ...item,
        pairs,
      };
    });
  }, [logs, groupBy]);

  const handleClickByPair = (value: string) => async (e: MouseEvent<HTMLDivElement>) => {
    e.stopPropagation();
    const isKeyValue = /(.+)?=(".+")/.test(value);
    const copyValue = isKeyValue ? `${value.replace(/=/, ": ")}` : `${groupBy}: "${value}"`;
    const isCopied = await copyToClipboard(copyValue);
    if (isCopied) {
      setCopied(value);
    }
  };

  const handleSelectGroupBy = (key: string) => () => {
    setGroupBy(key);
    searchParams.set("groupBy", key);
    setSearchParams(searchParams);
    handleCloseOptions();
  };

  const handleToggleExpandAll = useCallback(() => {
    setExpandGroups(new Array(groupData.length).fill(!expandAll));
  }, [expandAll]);

  const handleChangeExpand = (i: number) => (value: boolean) => {
    setExpandGroups((prev) => {
      const newExpandGroups = [...prev];
      newExpandGroups[i] = value;
      return newExpandGroups;
    });

  };

  useEffect(() => {
    if (copied === null) return;
    const timeout = setTimeout(() => setCopied(null), 2000);
    return () => clearTimeout(timeout);
  }, [copied]);

  useEffect(() => {
    setExpandGroups(new Array(groupData.length).fill(true));
  }, [groupData]);

  return (
    <>
      <div className="vm-group-logs">
        {groupData.map((item, i) => (
          <div
            className="vm-group-logs-section"
            key={item.keys.join("")}
          >
            <Accordion
              key={String(expandGroups[i])}
              defaultExpanded={expandGroups[i]}
              onChange={handleChangeExpand(i)}
              title={groupBy !== WITHOUT_GROUPING && (
                <div className="vm-group-logs-section-keys">
                  <span className="vm-group-logs-section-keys__title">Group by <code>{groupBy}</code>:</span>
                  {item.pairs.map((pair) => (
                    <Tooltip
                      title={copied === pair ? "Copied" : "Copy to clipboard"}
                      key={`${item.keys.join("")}_${pair}`}
                      placement={"top-center"}
                    >
                      <div
                        className={classNames({
                          "vm-group-logs-section-keys__pair": true,
                          "vm-group-logs-section-keys__pair_dark": isDarkTheme
                        })}
                        onClick={handleClickByPair(pair)}
                      >
                        {pair}
                      </div>
                    </Tooltip>
                  ))}
                  <span className="vm-group-logs-section-keys__count">{item.values.length} entries</span>
                </div>
              )}
            >
              <div className="vm-group-logs-section-rows">
                {item.values.map((value) => (
                  <GroupLogsItem
                    key={`${value._msg}${value._time}`}
                    log={value}
                  />
                ))}
              </div>
            </Accordion>
          </div>
        ))}
      </div>


      {settingsRef.current && React.createPortal((
        <div className="vm-group-logs-header">
          <Tooltip title={expandAll ? "Collapse All" : "Expand All"}>
            <Button
              variant="text"
              startIcon={expandAll ? <CollapseIcon/> : <ExpandIcon/> }
              onClick={handleToggleExpandAll}
              ariaLabel={expandAll ? "Collapse All" : "Expand All"}
            />
          </Tooltip>
          <Tooltip title={"Group by"}>
            <div ref={optionsButtonRef}>
              <Button
                variant="text"
                startIcon={<StorageIcon/> }
                onClick={toggleOpenOptions}
                ariaLabel={"Group by"}
              />
            </div>
          </Tooltip>
          {
            <Popper
              open={openOptions}
              placement="bottom-right"
              onClose={handleCloseOptions}
              buttonRef={optionsButtonRef}
            >
              <div className="vm-list vm-group-logs-header-keys">
                <div className="vm-group-logs-header-keys__search">
                  <TextField
                    label="Search key"
                    value={searchKey}
                    onChange={setSearchKey}
                    type="search"
                  />
                </div>
                {logsKeys.map(id => (
                  <div
                    className={classNames({
                      "vm-list-item": true,
                      "vm-list-item_active": id === groupBy
                    })}
                    key={id}
                    onClick={handleSelectGroupBy(id)}
                  >
                    {id}
                  </div>
                ))}
              </div>
            </Popper>
          }
        </div>
      ), settingsRef.current)}
    </>
  );
};

export default GroupLogs;
