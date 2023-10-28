import React, { FC, useEffect, useMemo, KeyboardEvent } from "react";
import { useFetchTopQueries } from "./hooks/useFetchTopQueries";
import Spinner from "../../components/Main/Spinner/Spinner";
import TopQueryPanel from "./TopQueryPanel/TopQueryPanel";
import { formatPrettyNumber } from "../../utils/uplot";
import { isSupportedDuration } from "../../utils/time";
import dayjs from "dayjs";
import { TopQueryStats } from "../../types";
import Button from "../../components/Main/Button/Button";
import { PlayIcon } from "../../components/Main/Icons";
import TextField from "../../components/Main/TextField/TextField";
import Alert from "../../components/Main/Alert/Alert";
import Tooltip from "../../components/Main/Tooltip/Tooltip";
import "./style.scss";
import useDeviceDetect from "../../hooks/useDeviceDetect";
import classNames from "classnames";
import useStateSearchParams from "../../hooks/useStateSearchParams";

const exampleDuration = "30ms, 15s, 3d4h, 1y2w";

const TopQueries: FC = () => {
  const { isMobile } = useDeviceDetect();

  const [topN, setTopN] = useStateSearchParams(10, "topN");
  const [maxLifetime, setMaxLifetime] = useStateSearchParams("10m", "maxLifetime");

  const { data, error, loading, fetch } = useFetchTopQueries({ topN, maxLifetime });

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
    if (typeof value === "number") return formatPrettyNumber(value, value, value);
    return value || key;
  };

  const onTopNChange = (value: string) => {
    setTopN(+value);
  };

  const onMaxLifetimeChange = (value: string) => {
    setMaxLifetime(value);
  };

  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key === "Enter") fetch();
  };

  useEffect(() => {
    if (!data) return;
    if (!topN) setTopN(+data.topN);
    if (!maxLifetime) setMaxLifetime(data.maxLifetime);
  }, [data]);

  useEffect(() => {
    fetch();
    window.addEventListener("popstate", fetch);

    return () => {
      window.removeEventListener("popstate", fetch);
    };
  }, []);

  return (
    <div
      className={classNames({
        "vm-top-queries": true,
        "vm-top-queries_mobile": isMobile,
      })}
    >
      {loading && <Spinner containerStyles={{ height: "500px" }}/>}

      <div
        className={classNames({
          "vm-top-queries-controls": true,
          "vm-block": true,
          "vm-block_mobile": isMobile,
        })}
      >
        <div className="vm-top-queries-controls-fields">
          <div className="vm-top-queries-controls-fields__item">
            <TextField
              label="Max lifetime"
              value={maxLifetime}
              error={errorMaxLife}
              helperText={`For example ${exampleDuration}`}
              onChange={onMaxLifetimeChange}
              onKeyDown={onKeyDown}
            />
          </div>
          <div className="vm-top-queries-controls-fields__item">
            <TextField
              label="Number of returned queries"
              type="number"
              value={topN || ""}
              error={errorTopN}
              onChange={onTopNChange}
              onKeyDown={onKeyDown}
            />
          </div>
        </div>
        <div
          className={classNames({
            "vm-top-queries-controls-bottom": true,
            "vm-top-queries-controls-bottom_mobile": isMobile,
          })}
        >
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
              onClick={fetch}
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
            rows={data.topBySumDuration}
            title={"Queries with most summary time to execute"}
            columns={[
              { key: "query" },
              { key: "sumDurationSeconds", title: "sum duration, sec" },
              { key: "timeRange", sortBy: "timeRangeSeconds", title: "query time interval" },
              { key: "count" }
            ]}
            defaultOrderBy={"sumDurationSeconds"}
          />
          <TopQueryPanel
            rows={data.topByAvgDuration}
            title={"Most heavy queries"}
            columns={[
              { key: "query" },
              { key: "avgDurationSeconds", title: "avg duration, sec" },
              { key: "timeRange", sortBy: "timeRangeSeconds", title: "query time interval" },
              { key: "count" }
            ]}
            defaultOrderBy={"avgDurationSeconds"}
          />
          <TopQueryPanel
            rows={data.topByCount}
            title={"Most frequently executed queries"}
            columns={[
              { key: "query" },
              { key: "timeRange", sortBy: "timeRangeSeconds", title: "query time interval" },
              { key: "count" }
            ]}
          />
        </div>
      </>)}
    </div>
  );
};

export default TopQueries;
