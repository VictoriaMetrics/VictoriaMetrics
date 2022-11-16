import React, { FC, useEffect, useMemo, KeyboardEvent } from "react";
import { useFetchTopQueries } from "./hooks/useFetchTopQueries";
import Spinner from "../../components/Main/Spinner/Spinner";
import TopQueryPanel from "./TopQueryPanel/TopQueryPanel";
import { useTopQueriesDispatch, useTopQueriesState } from "../../state/topQueries/TopQueriesStateContext";
import { formatPrettyNumber } from "../../utils/uplot/helpers";
import { isSupportedDuration } from "../../utils/time";
import dayjs from "dayjs";
import { TopQueryStats } from "../../types";
import { useSetQueryParams } from "./hooks/useSetQueryParams";
import Button from "../../components/Main/Button/Button";
import { PlayIcon } from "../../components/Main/Icons";
import TextField from "../../components/Main/TextField/TextField";
import Alert from "../../components/Main/Alert/Alert";
import Tooltip from "../../components/Main/Tooltip/Tooltip";
import "./style.scss";

const exampleDuration = "30ms, 15s, 3d4h, 1y2w";

const Index: FC = () => {
  const { data, error, loading } = useFetchTopQueries();
  const { topN, maxLifetime } = useTopQueriesState();
  const topQueriesDispatch = useTopQueriesDispatch();
  useSetQueryParams();

  const maxLifetimeValid = useMemo(() => {
    const durItems = maxLifetime.trim().split(" ");
    const durObject = durItems.reduce((prev, curr) => {
      const dur = isSupportedDuration(curr);
      return dur ? { ...prev, ...dur } : { ...prev };
    }, {});
    const delta = dayjs.duration(durObject).asMilliseconds();
    return !!delta;
  }, [maxLifetime]);

  const invalidTopN = useMemo(() => !!topN && topN < 1, [topN]);
  const errorTopN = useMemo(() => invalidTopN ? "Number must be bigger than zero" : "", [invalidTopN]);
  const errorMaxLife = useMemo(() => !maxLifetimeValid ? "Invalid duration value" : "", [maxLifetimeValid]);

  const getQueryStatsTitle = (key: keyof TopQueryStats) => {
    if (!data) return key;
    const value = data[key];
    if (typeof value === "number") return formatPrettyNumber(value);
    return value || key;
  };

  const onTopNChange = (value: string) => {
    topQueriesDispatch({ type: "SET_TOP_N", payload: +value });
  };

  const onMaxLifetimeChange = (value: string) => {
    topQueriesDispatch({ type: "SET_MAX_LIFE_TIME", payload: value });
  };

  const onApplyQuery = () => {
    topQueriesDispatch({ type: "SET_RUN_QUERY" });
  };

  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key === "Enter") onApplyQuery();
  };

  useEffect(() => {
    if (!data) return;
    if (!topN) topQueriesDispatch({ type: "SET_TOP_N", payload: +data.topN });
    if (!maxLifetime) topQueriesDispatch({ type: "SET_MAX_LIFE_TIME", payload: data.maxLifetime });
  }, [data]);

  return (
    <div className="vm-top-queries">
      {loading && <Spinner containerStyles={{ height: "500px" }}/>}

      <div className="vm-top-queries-controls vm-block">
        <div className="vm-top-queries-controls__fields">
          <TextField
            label="Max lifetime"
            value={maxLifetime}
            error={errorMaxLife}
            helperText={`For example ${exampleDuration}`}
            onChange={onMaxLifetimeChange}
            onKeyDown={onKeyDown}
          />
          <TextField
            label="Number of returned queries"
            type="number"
            value={topN || ""}
            error={errorTopN}
            onChange={onTopNChange}
            onKeyDown={onKeyDown}
          />
        </div>
        <div className="vm-top-queries-controls-bottom">
          <div className="vm-top-queries-controls-bottom__info">
            VictoriaMetrics tracks the last&nbsp;
            <Tooltip title="search.queryStats.lastQueriesCount">
              <b>
                {getQueryStatsTitle("search.queryStats.lastQueriesCount")}
              </b>
            </Tooltip>
            &nbsp;queries with durations at least&nbsp;
            <Tooltip title="search.queryStats.minQueryDuration">
              <b>
                {getQueryStatsTitle("search.queryStats.minQueryDuration")}
              </b>
            </Tooltip>
          </div>
          <div className="vm-top-queries-controls-bottom__button">
            <Button
              startIcon={<PlayIcon/>}
              onClick={onApplyQuery}
            >
              Execute
            </Button>
          </div>
        </div>
      </div>

      {error && <Alert variant="error">{error}</Alert>}

      {data && (<>
        <div className="vm-top-queries-panels">
          <TopQueryPanel
            rows={data.topByCount}
            title={"Most frequently executed queries"}
            columns={[
              { key: "query" },
              { key: "timeRangeHours", title: "time range, hours" },
              { key: "count" }
            ]}
          />
          <TopQueryPanel
            rows={data.topByAvgDuration}
            title={"Most heavy queries"}
            columns={[
              { key: "query" },
              { key: "avgDurationSeconds", title: "avg duration, seconds" },
              { key: "timeRangeHours", title: "time range, hours" },
              { key: "count" }
            ]}
            defaultOrderBy={"avgDurationSeconds"}
          />
          <TopQueryPanel
            rows={data.topBySumDuration}
            title={"Queries with most summary time to execute"}
            columns={[
              { key: "query" },
              { key: "sumDurationSeconds", title: "sum duration, seconds" },
              { key: "timeRangeHours", title: "time range, hours" },
              { key: "count" }
            ]}
            defaultOrderBy={"sumDurationSeconds"}
          />
        </div>
      </>)}
    </div>
  );
};

export default Index;
