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
import { PlayCircleOutlineIcon } from "../../components/Main/Icons";
import TextField from "../../components/Main/TextField/TextField";

const exampleDuration = "30ms, 15s, 3d4h, 1y2w";

const Index: FC = () => {
  const { data, error, loading } = useFetchTopQueries();
  const { topN, maxLifetime } = useTopQueriesState();
  const topQueriesDispatch = useTopQueriesDispatch();
  useSetQueryParams();

  const invalidTopN = useMemo(() => !!topN && topN < 1, [topN]);

  const maxLifetimeValid = useMemo(() => {
    const durItems = maxLifetime.trim().split(" ");
    const durObject = durItems.reduce((prev, curr) => {
      const dur = isSupportedDuration(curr);
      return dur ? { ...prev, ...dur } : { ...prev };
    }, {});
    const delta = dayjs.duration(durObject).asMilliseconds();
    return !!delta;
  }, [maxLifetime]);

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
    <div
      style={{ minHeight: "calc(100vh - 64px)" }}
    >
      {loading && <Spinner containerStyles={{ height: "500px" }}/>}

      <div >
        <div >
          <div >
            <TextField
              label="Max lifetime"
              // size="medium"
              value={maxLifetime}
              error={!maxLifetimeValid ? "Invalid duration value" : `For example ${exampleDuration}`}
              onChange={onMaxLifetimeChange}
              onKeyDown={onKeyDown}
            />
          </div>
          <div>
            <TextField
              label="Number of returned queries"
              type="number"
              // size="medium"
              value={topN || ""}
              error={invalidTopN ? "Number must be bigger than zero" : " "}
              onChange={onTopNChange}
              onKeyDown={onKeyDown}
            />
          </div>
          <div>
            {/*<Tooltip title="Apply">*/}
            <Button
              onClick={onApplyQuery}
            >
              <PlayCircleOutlineIcon/>
            </Button>
            {/*</Tooltip>*/}
          </div>
        </div>
        <div>
            VictoriaMetrics tracks the last&nbsp;
          {/*<Tooltip*/}
          {/*  arrow*/}
          {/*  title={<Typography>search.queryStats.lastQueriesCount</Typography>}*/}
          {/*>*/}
          <b style={{ cursor: "default" }}>
            {getQueryStatsTitle("search.queryStats.lastQueriesCount")}
          </b>
          {/*</Tooltip>*/}
            &nbsp;queries with durations at least&nbsp;
          {/*<Tooltip*/}
          {/*  arrow*/}
          {/*  title={<Typography>search.queryStats.minQueryDuration</Typography>}*/}
          {/*>*/}
          <b style={{ cursor: "default" }}>
            {getQueryStatsTitle("search.queryStats.minQueryDuration")}
          </b>
          {/*</Tooltip>*/}
        </div>
      </div>

      {/* TODO add alert */}
      {/*{error && <Alert*/}
      {/*  color="error"*/}
      {/*  severity="error"*/}
      {/*  sx={{ whiteSpace: "pre-wrap", my: 2 }}*/}
      {/*>{error}</Alert>}*/}

      {data && (<>
        <div>
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
            // columns={["query", "avgDurationSeconds", "timeRangeHours", "count"]}
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
