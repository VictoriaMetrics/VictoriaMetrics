import React, { FC, useCallback, useEffect, useMemo, useState } from "preact/compat";
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
import { LOGS_DISPLAY_FIELDS, LOGS_GROUP_BY, LOGS_URL_PARAMS } from "../../../constants/logs";
import Pagination from "../../../components/Main/Pagination/Pagination";
import SelectLimit from "../../../components/Main/Pagination/SelectLimit/SelectLimit";
import { usePaginateGroups } from "../hooks/usePaginateGroups";
import { GroupLogsType } from "../../../types";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import DownloadLogsButton from "../DownloadLogsButton/DownloadLogsButton";
import { hasSortPipe } from "../../../components/Configurators/QueryEditor/LogsQL/utils/sort";

interface Props {
  logs: Logs[];
  settingsRef: React.RefObject<HTMLElement>;
}

const GroupLogs: FC<Props> = ({ logs, settingsRef }) => {
  const { isMobile } = useDeviceDetect();
  const [searchParams, setSearchParams] = useSearchParams();

  const query = searchParams.get("query") || "";
  const queryHasSort = hasSortPipe(query);

  const [page, setPage] = useState(1);
  const [expandGroups, setExpandGroups] = useState<boolean[]>([]);

  const groupBy = searchParams.get(LOGS_URL_PARAMS.GROUP_BY) || LOGS_GROUP_BY;
  const displayFieldsString = searchParams.get(LOGS_URL_PARAMS.DISPLAY_FIELDS) || LOGS_DISPLAY_FIELDS;
  const displayFields = useMemo(() => displayFieldsString.split(","), [displayFieldsString]);

  const rowsPerPageRaw = Number(searchParams.get(LOGS_URL_PARAMS.ROWS_PER_PAGE));
  const rowsPerPage = isNaN(rowsPerPageRaw) ? 0 : rowsPerPageRaw;

  const expandAll = useMemo(() => expandGroups.every(Boolean), [expandGroups]);

  const groupData: GroupLogsType[] = useMemo(() => {
    return groupByMultipleKeys(logs, [groupBy]).map((item) => {
      const streamValue = item.values[0]?.[groupBy] || "";
      const pairs = getStreamPairs(streamValue);

      // VictoriaLogs sends rows oldest → newest when the query has no `| sort` pipe,
      // so we reverse the array to put the newest entries first.
      // If a sort is already specified, keep the original order.
      const values = queryHasSort ? item.values : item.values.toReversed();

      return {
        keys: item.keys,
        keysString: item.keys.join(""),
        values,
        pairs,
        total: values.length,
      };
    }).sort((a, b) => b.total - a.total); // groups sorting
  }, [logs, groupBy, queryHasSort]);

  const paginatedGroups = usePaginateGroups(groupData, page, rowsPerPage);

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

  const handleSetRowsPerPage = (limit?: number) => {
    if (limit) {
      searchParams.set(LOGS_URL_PARAMS.ROWS_PER_PAGE, String(limit));
    } else {
      searchParams.delete(LOGS_URL_PARAMS.ROWS_PER_PAGE);
    }

    setSearchParams(searchParams);
  };

  const handlePageChange = (newPage: number) => {
    setPage(newPage);
    window.scrollTo({ top: 0 });
  };

  const getLogs = useCallback(() => logs, [logs]);

  useEffect(() => {
    setExpandGroups(new Array(groupData.length).fill(!isMobile));
  }, [groupData]);

  useEffect(() => {
    setPage(1);
  }, [rowsPerPage]);

  return (
    <>
      <div className="vm-group-logs">
        {paginatedGroups.map((group, groupN) => (
          <div
            className="vm-group-logs-section"
            key={group.keysString}
          >
            <Accordion
              defaultExpanded={expandGroups[groupN]}
              onChange={handleChangeExpand(groupN)}
              title={(
                <GroupLogsHeader
                  group={group}
                  index={groupN}
                />
              )}
            >
              <div className="vm-group-logs-section-rows">
                {group.values.map((log, rowN) => (
                  <GroupLogsItem
                    key={`${groupN}_${rowN}_${log._time}`}
                    log={log}
                    displayFields={displayFields}
                  />
                ))}
              </div>
            </Accordion>
          </div>
        ))}

        <Pagination
          currentPage={page}
          totalItems={logs.length}
          itemsPerPage={rowsPerPage || Infinity}
          onPageChange={handlePageChange}
        />
      </div>


      {settingsRef.current && React.createPortal((
        <div className="vm-group-logs-header">
          <div className="vm-explore-logs-body-header__log-info">
            Total groups: <b>{groupData.length}</b>
          </div>
          <SelectLimit
            allowUnlimited
            limit={rowsPerPage}
            onChange={handleSetRowsPerPage}
          />
          <Tooltip title={expandAll ? "Collapse All" : "Expand All"}>
            <Button
              variant="text"
              startIcon={expandAll ? <CollapseIcon/> : <ExpandIcon/>}
              onClick={handleToggleExpandAll}
              ariaLabel={expandAll ? "Collapse All" : "Expand All"}
            />
          </Tooltip>
          <DownloadLogsButton getLogs={getLogs}/>
          <GroupLogsConfigurators logs={logs}/>
        </div>
      ), settingsRef.current)}
    </>
  );
};

export default GroupLogs;
