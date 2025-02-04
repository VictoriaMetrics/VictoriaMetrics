import React, { FC, useCallback, useEffect, useMemo } from "preact/compat";
import { useState } from "react";
import "./style.scss";
import { Logs } from "../../../api/types";
import Accordion from "../../../components/Main/Accordion/Accordion";
import { groupByMultipleKeys } from "../../../utils/array";
import Tooltip from "../../../components/Main/Tooltip/Tooltip";
import GroupLogsItem from "./GroupLogsItem";
import Button from "../../../components/Main/Button/Button";
import { CollapseIcon, ExpandIcon } from "../../../components/Main/Icons";
import { useSearchParams } from "react-router-dom";
import { getStreamPairs } from "../../../utils/logs";
import GroupLogsConfigurators
  from "../../../components/LogsConfigurators/GroupLogsConfigurators/GroupLogsConfigurators";
import GroupLogsHeader from "./GroupLogsHeader";
import { LOGS_DISPLAY_FIELDS, LOGS_GROUP_BY, LOGS_URL_PARAMS, WITHOUT_GROUPING } from "../../../constants/logs";

interface Props {
  logs: Logs[];
  settingsRef: React.RefObject<HTMLElement>;
}

const GroupLogs: FC<Props> = ({ logs, settingsRef }) => {
  const [searchParams] = useSearchParams();

  const [expandGroups, setExpandGroups] = useState<boolean[]>([]);

  const groupBy = searchParams.get(LOGS_URL_PARAMS.GROUP_BY) || LOGS_GROUP_BY;
  const displayFieldsString = searchParams.get(LOGS_URL_PARAMS.DISPLAY_FIELDS) || LOGS_DISPLAY_FIELDS;
  const displayFields = displayFieldsString.split(",");

  const expandAll = useMemo(() => expandGroups.every(Boolean), [expandGroups]);

  const groupData = useMemo(() => {
    return groupByMultipleKeys(logs, [groupBy]).map((item) => {
      const streamValue = item.values[0]?.[groupBy] || "";
      const pairs = getStreamPairs(streamValue);
      // values sorting by time
      const values = item.values.sort((a, b) => new Date(b._time).getTime() - new Date(a._time).getTime());
      return {
        keys: item.keys,
        keysString: item.keys.join(""),
        values,
        pairs,
      };
    }).sort((a, b) => b.values.length - a.values.length); // groups sorting
  }, [logs, groupBy]);

  const handleToggleExpandAll = useCallback(() => {
    setExpandGroups(new Array(groupData.length).fill(!expandAll));
  }, [expandAll, groupData.length]);

  const handleChangeExpand = useCallback((i: number) => (value: boolean) => {
    setExpandGroups((prev) => {
      const newExpandGroups = [...prev];
      newExpandGroups[i] = value;
      return newExpandGroups;
    });
  }, []);


  useEffect(() => {
    setExpandGroups(new Array(groupData.length).fill(true));
  }, [groupData]);

  return (
    <>
      <div className="vm-group-logs">
        {groupData.map((item, i) => (
          <div
            className="vm-group-logs-section"
            key={item.keysString}
          >
            <Accordion
              defaultExpanded={expandGroups[i]}
              onChange={handleChangeExpand(i)}
              title={groupBy !== WITHOUT_GROUPING && <GroupLogsHeader group={item}/>}
            >
              <div className="vm-group-logs-section-rows">
                {item.values.map((value) => (
                  <GroupLogsItem
                    key={`${value._msg}${value._time}`}
                    log={value}
                    displayFields={displayFields}
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
              startIcon={expandAll ? <CollapseIcon/> : <ExpandIcon/>}
              onClick={handleToggleExpandAll}
              ariaLabel={expandAll ? "Collapse All" : "Expand All"}
            />
          </Tooltip>
          <GroupLogsConfigurators logs={logs}/>
        </div>
      ), settingsRef.current)}
    </>
  );
};

export default GroupLogs;
