import { useMemo, Fragment } from "react";
import { Link } from "react-router-dom";
import { TopQueryColumn } from "../TopQueryPanel/TopQueryPanel";
import { humanizeSeconds } from "../../../utils/time";
import { formatBytes } from "../../../utils/bytes";
import router from "../../../router";

type AlertRuleRef = { group_id: string; rule_id: string };

type UseTopQueriesColumns = {
  maxLifetime: string;
  alertQueries?: Map<string, AlertRuleRef>;
};

export const useTopQueriesColumns = ({ maxLifetime, alertQueries }: UseTopQueriesColumns) => {
  return useMemo(() => {
    const queryCol: TopQueryColumn = {
      key: "query"
    };

    const timeRangeCol: TopQueryColumn = {
      key: "timeRange",
      sortBy: "timeRangeSeconds",
      title: "range",
      tooltip: (
        <Fragment>
          The time range between start and end of the query request.<br/>
          <br/>
          &apos;instant&apos; means the query was executed at a single point in time without a time range.<br/>
          <br/>
          &apos;alert?&apos; means the query matches an alert rule — the link goes to the first matching alert.
        </Fragment>
      ),
      format: (row) => {
        const ref = alertQueries?.get(row.query);
        if (ref) {
          return (
            <Link
              to={`${router.rules}?group_id=${ref.group_id}&rule_id=${ref.rule_id}`}
              className="vm-link vm-link_colored"
            >
              alert?
            </Link>
          );
        }
        return row.timeRange;
      },
    };

    const countCol: TopQueryColumn = {
      key: "count",
      tooltip: `The number of times the query was executed over the last ${maxLifetime}`,
    };

    const topBySumDuration: TopQueryColumn[] = [
      queryCol,
      {
        key: "sumDurationSeconds",
        title: "duration",
        tooltip: `Cumulative time spent executing the query across all its invocations over the last ${maxLifetime}`,
        format: (row) => humanizeSeconds(row.sumDurationSeconds)
      },
      timeRangeCol,
      countCol,
    ];

    const topByAvgDuration: TopQueryColumn[] = [
      queryCol,
      {
        key: "avgDurationSeconds",
        title: "duration",
        tooltip: `Average time spent executing the query over the last ${maxLifetime}`,
        format: (row) => humanizeSeconds(row.avgDurationSeconds)
      },
      timeRangeCol,
      countCol,
    ];

    const topByCount: TopQueryColumn[] = [
      queryCol,
      timeRangeCol,
      countCol,
    ];

    const topByAvgMemoryUsage: TopQueryColumn[] = [
      queryCol,
      {
        key: "avgMemoryBytes",
        title: "memory",
        tooltip: `Average memory used during query execution over the last ${maxLifetime}`,
        format: (row) => formatBytes(row.avgMemoryBytes)
      },
      timeRangeCol,
      countCol,
    ];

    return {
      topBySumDuration,
      topByAvgDuration,
      topByCount,
      topByAvgMemoryUsage,
    };
  }, [maxLifetime, alertQueries]);
};
