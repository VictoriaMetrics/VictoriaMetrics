import { useMemo } from "react";
import { TopQueryColumn } from "../TopQueryPanel/TopQueryPanel";
import { humanizeSeconds } from "../../../utils/time";
import { formatBytes } from "../../../utils/bytes";

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
  }, [maxLifetime]);
};
