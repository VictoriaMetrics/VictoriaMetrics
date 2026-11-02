import { useMemo } from "react";
import { TopQueryColumn } from "../TopQueryPanel/TopQueryPanel";
import { humanizeSeconds } from "../../../utils/time";
import { formatBytes } from "../../../utils/bytes";
import { formatTenant } from "../../../utils/tenants";

type UseTopQueriesColumns = {
  maxLifetime: string;
};

export const useTopQueriesColumns = ({ maxLifetime }: UseTopQueriesColumns) => {
  return useMemo(() => {
    const queryCol: TopQueryColumn = {
      key: "query"
    };

    const timeRangeCol: TopQueryColumn = {
      key: "timeRange",
      sortBy: "timeRangeSeconds",
      title: "range",
      tooltip: "The time range between start and end of the query request. 'instant' means the query was executed at a single point in time without a time range"
    };

    const countCol: TopQueryColumn = {
      key: "count",
      tooltip: `The number of times the query was executed over the last ${maxLifetime}`,
    };

    const tenantCol: TopQueryColumn = {
      key: "accountID",
      title: "tenant",
      sortable: false,
      visible: row => !!formatTenant(row),
      format: formatTenant,
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
      tenantCol,
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
      tenantCol,
    ];

    const topByCount: TopQueryColumn[] = [
      queryCol,
      timeRangeCol,
      countCol,
      tenantCol,
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
      tenantCol,
    ];

    return {
      topBySumDuration,
      topByAvgDuration,
      topByCount,
      topByAvgMemoryUsage,
    };
  }, [maxLifetime]);
};
