import React, {ChangeEvent, FC, useEffect, useMemo, KeyboardEvent} from "react";
import Box from "@mui/material/Box";
import {useFetchTopQueries} from "../../hooks/useFetchTopQueries";
import Spinner from "../common/Spinner";
import Alert from "@mui/material/Alert";
import TopQueryPanel from "./TopQueryPanel/TopQueryPanel";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";
import TextField from "@mui/material/TextField";
import {useTopQueriesDispatch, useTopQueriesState} from "../../state/topQueries/TopQueriesStateContext";
import {formatPrettyNumber} from "../../utils/uplot/helpers";
import {isSupportedDuration} from "../../utils/time";
import IconButton from "@mui/material/IconButton";
import PlayCircleOutlineIcon from "@mui/icons-material/PlayCircleOutline";
import RestartAltIcon from "@mui/icons-material/RestartAlt";
import dayjs from "dayjs";

const exampleDuration = "30ms, 15s, 3d4h, 1y2w";

const TopQueries: FC = () => {
  const {data, error, loading} = useFetchTopQueries();
  const {topN, maxLifetime} = useTopQueriesState();
  const topQueriesDispatch = useTopQueriesDispatch();

  const maxLifetimeValid = useMemo(() => {
    const durItems = maxLifetime.trim().split(" ");
    const durObject = durItems.reduce((prev, curr) => {
      const dur = isSupportedDuration(curr);
      return dur ? {...prev, ...dur} : {...prev};
    }, {});
    const delta = dayjs.duration(durObject).asMilliseconds();
    return !!delta;
  }, [maxLifetime]);

  const onTopNChange = (e: ChangeEvent<HTMLTextAreaElement|HTMLInputElement>) => {
    topQueriesDispatch({type: "SET_TOP_N", payload: +e.target.value});
  };

  const onMaxLifetimeChange = (e: ChangeEvent<HTMLTextAreaElement|HTMLInputElement>) => {
    topQueriesDispatch({type: "SET_MAX_LIFE_TIME", payload: e.target.value});
  };

  const onApplyQuery = () => {
    topQueriesDispatch({type: "SET_RUN_QUERY"});
  };

  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key === "Enter") onApplyQuery();
  };

  useEffect(() => {
    if (!data) return;
    if (!topN) topQueriesDispatch({type: "SET_TOP_N", payload: +data.topN});
    if (!maxLifetime) topQueriesDispatch({type: "SET_MAX_LIFE_TIME", payload: data.maxLifetime});
  }, [data]);

  return (
    <Box p={4} style={{minHeight: "calc(100vh - 64px)"}}>
      {loading && <Spinner isLoading={true} height={"100%"}/>}

      <Box boxShadow="rgba(99, 99, 99, 0.2) 0px 2px 8px 0px;" p={4} pb={2} m={-4} mb={4}>
        <Box display={"flex"} alignItems={"flex"} mb={2}>
          <Box mr={2} flexGrow={1}>
            <TextField
              fullWidth
              label="Max lifetime"
              size="medium"
              variant="outlined"
              value={maxLifetime}
              error={!maxLifetimeValid}
              helperText={!maxLifetimeValid ? "Invalid duration value" : `For example ${exampleDuration}`}
              onChange={onMaxLifetimeChange}
              onKeyDown={onKeyDown}
            />
          </Box>
          <Box mr={2}>
            <TextField
              fullWidth
              label="Number of returned queries"
              type="number"
              size="medium"
              variant="outlined"
              value={topN}
              error={!!(topN && topN < 1)}
              helperText={topN && topN < 1 ? "Number must be bigger than zero" : " "}
              onChange={onTopNChange}
              onKeyDown={onKeyDown}
            />
          </Box>
          <Box>
            <Tooltip title="Apply">
              <IconButton onClick={onApplyQuery} sx={{height: "49px", width: "49px"}}>
                <PlayCircleOutlineIcon/>
              </IconButton>
            </Tooltip>
          </Box>
        </Box>
        <Typography variant="body1" pt={2}>
            VictoriaMetrics tracks the last&nbsp;
          <Tooltip arrow title={<Typography>search.queryStats.lastQueriesCount</Typography>}>
            <b style={{cursor: "default"}}>
              {data ? formatPrettyNumber(data["search.queryStats.lastQueriesCount"]) : "search.queryStats.lastQueriesCount"}
            </b>
          </Tooltip>
            &nbsp;queries with durations at least&nbsp;
          <Tooltip arrow title={<Typography>search.queryStats.minQueryDuration</Typography>}>
            <b style={{cursor: "default"}}>
              {data ? data["search.queryStats.minQueryDuration"] : "search.queryStats.minQueryDuration"}
            </b>
          </Tooltip>
        </Typography>
      </Box>

      {error && <Alert color="error" severity="error" sx={{whiteSpace: "pre-wrap", my: 2}}>{error}</Alert>}

      {data && (<>
        <Box>
          <TopQueryPanel
            rows={data.topByCount}
            title={"Top by count"}
            description={"The most frequently executed queries"}
          />
          <TopQueryPanel
            rows={data.topByAvgDuration}
            title={"Top by avg duration"}
            description={"Queries that took the most average execution time"}
          />
          <TopQueryPanel
            rows={data.topBySumDuration}
            title={"Top by sum duration"}
            description={"Queries that took the highest summary execution time"}
          />
        </Box>
      </>)}
    </Box>
  );
};

export default TopQueries;
