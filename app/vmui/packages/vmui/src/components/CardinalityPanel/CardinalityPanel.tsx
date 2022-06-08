import React, {ChangeEvent, FC, useState} from "react";
import {SyntheticEvent} from "react";
import {Typography, Grid, Alert, Box, Tabs, Tab, Tooltip} from "@mui/material";
import TableChartIcon from "@mui/icons-material/TableChart";
import ShowChartIcon from "@mui/icons-material/ShowChart";
import {useFetchQuery} from "../../hooks/useCardinalityFetch";
import EnhancedTable from "../Table/Table";
import {TSDBStatus, TopHeapEntry, DefaultState, Tabs as TabsType, Containers} from "./types";
import {
  defaultHeadCells,
  headCellsWithProgress,
  SPINNER_TITLE,
  spinnerContainerStyles
} from "./consts";
import {defaultProperties, progressCount, queryUpdater, tableTitles} from "./helpers";
import {Data} from "../Table/types";
import BarChart from "../BarChart/BarChart";
import CardinalityConfigurator from "./CardinalityConfigurator/CardinalityConfigurator";
import {barOptions} from "../BarChart/consts";
import Spinner from "../common/Spinner";
import TabPanel from "../TabPanel/TabPanel";
import {useCardinalityDispatch, useCardinalityState} from "../../state/cardinality/CardinalityStateContext";
import {tableCells} from "./TableCells/TableCells";

const CardinalityPanel: FC = () => {
  const cardinalityDispatch = useCardinalityDispatch();

  const {topN, match, date} = useCardinalityState();
  const configError = "";
  const [query, setQuery] = useState(match || "");
  const [queryHistoryIndex, setQueryHistoryIndex] = useState(0);
  const [queryHistory, setQueryHistory] = useState<string[]>([]);

  const onRunQuery = () => {
    setQueryHistory(prev => [...prev, query]);
    setQueryHistoryIndex(prev => prev + 1);
    cardinalityDispatch({type: "SET_MATCH", payload: query});
    cardinalityDispatch({type: "RUN_QUERY"});
  };

  const onSetQuery = (query: string) => {
    setQuery(query);
  };

  const onSetHistory = (step: number) => {
    const newIndexHistory = queryHistoryIndex + step;
    if (newIndexHistory < 0 || newIndexHistory >= queryHistory.length) return;
    setQueryHistoryIndex(newIndexHistory);
    setQuery(queryHistory[newIndexHistory]);
  };

  const onTopNChange = (e: ChangeEvent<HTMLTextAreaElement|HTMLInputElement>) => {
    cardinalityDispatch({type: "SET_TOP_N", payload: +e.target.value});
  };

  const {isLoading, tsdbStatus, error} = useFetchQuery();
  const defaultProps = defaultProperties(tsdbStatus);
  const [stateTabs, setTab] = useState(defaultProps.defaultState);

  const handleTabChange = (e: SyntheticEvent, newValue: number) => {
    // eslint-disable-next-line @typescript-eslint/ban-ts-comment
    // @ts-ignore
    setTab({...stateTabs, [e.target.id]: newValue});
  };

  const handleFilterClick = (key: string) => (e: SyntheticEvent) => {
    const name = e.currentTarget.id;
    const query = queryUpdater[key](name);
    setQuery(query);
    setQueryHistory(prev => [...prev, query]);
    setQueryHistoryIndex(prev => prev + 1);
    cardinalityDispatch({type: "SET_MATCH", payload: query});
    cardinalityDispatch({type: "RUN_QUERY"});
  };

  return (
    <>
      {isLoading && <Spinner
        isLoading={isLoading}
        height={"800px"}
        containerStyles={spinnerContainerStyles("100%")}
        title={<Alert color="error" severity="error" sx={{whiteSpace: "pre-wrap", mt: 2}}>
          {SPINNER_TITLE}
        </Alert>}
      />}
      <CardinalityConfigurator error={configError} query={query} onRunQuery={onRunQuery} onSetQuery={onSetQuery}
        onSetHistory={onSetHistory} onTopNChange={onTopNChange} topN={topN} />
      {error && <Alert color="error" severity="error" sx={{whiteSpace: "pre-wrap", mt: 2}}>{error}</Alert>}
      {<Box m={2}>
        Analyzed <b>{tsdbStatus.totalSeries}</b> series and <b>{tsdbStatus.totalLabelValuePairs}</b> label=value pairs
        at <b>{date}</b> {match && <span>for series selector <b>{match}</b></span>}. Show top {topN} entries per table.
      </Box>}
      {Object.keys(tsdbStatus).map((key ) => {
        if (key == "totalSeries" || key == "totalLabelValuePairs") return null;
        const tableTitle = tableTitles[key];
        const rows = tsdbStatus[key as keyof TSDBStatus] as unknown as Data[];
        rows.forEach((row) => {
          progressCount(tsdbStatus.totalSeries, key, row);
          row.actions = "0";
        });
        const headerCells = (key == "seriesCountByMetricName" || key == "seriesCountByLabelValuePair") ? headCellsWithProgress : defaultHeadCells;
        return (
          <>
            <Grid container spacing={2} sx={{px: 2}}>
              <Grid item xs={12} md={12} lg={12} key={key}>
                <Typography gutterBottom variant="h5" component="h5">
                  {tableTitle}
                </Typography>
                <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
                  <Tabs
                    value={stateTabs[key as keyof DefaultState]}
                    onChange={handleTabChange} aria-label="basic tabs example">
                    {defaultProps.tabs[key as keyof TabsType].map((title: string, i: number) =>
                      <Tab
                        key={title}
                        label={title}
                        aria-controls={`tabpanel-${i}`}
                        id={key}
                        iconPosition={"start"}
                        icon={ i === 0 ? <TableChartIcon /> : <ShowChartIcon /> } />
                    )}
                  </Tabs>
                </Box>
                {defaultProps.tabs[key as keyof TabsType].map((_,idx) =>
                  <div
                    ref={defaultProps.containerRefs[key as keyof Containers<HTMLDivElement>]}
                    style={{width: "100%", paddingRight: idx !== 0 ? "40px" : 0 }} key={`${key}-${idx}`}>
                    <TabPanel value={stateTabs[key as keyof DefaultState]} index={idx}>
                      {stateTabs[key as keyof DefaultState] === 0 ? <EnhancedTable
                        rows={rows}
                        headerCells={headerCells}
                        defaultSortColumn={"value"}
                        tableCells={(row) => tableCells(row,date,handleFilterClick(key))}
                      />: <BarChart
                        data={[
                          // eslint-disable-next-line @typescript-eslint/ban-ts-comment
                          // @ts-ignore
                          rows.map((v) => v.name),
                          rows.map((v) => v.value),
                          rows.map((_, i) => i % 12 == 0 ? 1 : i % 10 == 0 ? 2 : 0),
                        ]}
                        container={defaultProps.containerRefs[key as keyof Containers<HTMLDivElement>]?.current}
                        configs={barOptions}
                      />}
                    </TabPanel>
                  </div>
                )}
              </Grid>
            </Grid>
          </>
        );
      })}
    </>
  );
};

export default CardinalityPanel;
