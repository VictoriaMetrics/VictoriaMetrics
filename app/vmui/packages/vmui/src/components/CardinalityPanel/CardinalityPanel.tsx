import React, {ChangeEvent, FC, useState} from "react";
import {SyntheticEvent} from "react";
import {Alert} from "@mui/material";
import {useFetchQuery} from "../../hooks/useCardinalityFetch";
import {
  LABEL_VALUE_PAIR_CONTENT_TITLE,
  LABEL_VALUE_PAIRS_TABLE_HEADERS,
  LABEL_WITH_UNIQUE_VALUES_TABLE_HEADERS,
  LABELS_CONTENT_TITLE, METRICS_TABLE_HEADERS,
  SERIES_CONTENT_TITLE,
  SPINNER_TITLE,
  spinnerContainerStyles
} from "./consts";
import {defaultProperties, queryUpdater} from "./helpers";
import {Data} from "../Table/types";
import CardinalityConfigurator from "./CardinalityConfigurator/CardinalityConfigurator";
import Spinner from "../common/Spinner";
import {useCardinalityDispatch, useCardinalityState} from "../../state/cardinality/CardinalityStateContext";
import MetricsContent from "./MetricsContent/MetricsContent";

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
        onSetHistory={onSetHistory} onTopNChange={onTopNChange} topN={topN} date={date} match={match}
        totalSeries={tsdbStatus.totalSeries} totalLabelValuePairs={tsdbStatus.totalLabelValuePairs}/>
      {error && <Alert color="error" severity="error" sx={{whiteSpace: "pre-wrap", mt: 2}}>{error}</Alert>}
      <MetricsContent
        activeTab={stateTabs.seriesCountByMetricName}
        rows={tsdbStatus.seriesCountByMetricName as unknown as Data[]}
        onChange={handleTabChange}
        onActionClick={handleFilterClick("seriesCountByMetricName")}
        tabs={defaultProps.tabs.seriesCountByMetricName}
        chartContainer={defaultProps.containerRefs.seriesCountByMetricName}
        totalSeries={tsdbStatus.totalSeries}
        tabId={"seriesCountByMetricName"}
        sectionTitle={SERIES_CONTENT_TITLE}
        tableHeaderCells={METRICS_TABLE_HEADERS}
      />
      <MetricsContent
        activeTab={stateTabs.seriesCountByLabelValuePair}
        rows={tsdbStatus.seriesCountByLabelValuePair as unknown as Data[]}
        onChange={handleTabChange}
        onActionClick={handleFilterClick("seriesCountByLabelValuePair")}
        tabs={defaultProps.tabs.seriesCountByLabelValuePair}
        chartContainer={defaultProps.containerRefs.seriesCountByLabelValuePair}
        totalSeries={tsdbStatus.totalLabelValuePairs}
        tabId={"seriesCountByLabelValuePair"}
        sectionTitle={LABEL_VALUE_PAIR_CONTENT_TITLE}
        tableHeaderCells={LABEL_VALUE_PAIRS_TABLE_HEADERS}
      />
      <MetricsContent
        activeTab={stateTabs.labelValueCountByLabelName}
        rows={tsdbStatus.labelValueCountByLabelName as unknown as Data[]}
        onChange={handleTabChange}
        onActionClick={handleFilterClick("labelValueCountByLabelName")}
        tabs={defaultProps.tabs.labelValueCountByLabelName}
        chartContainer={defaultProps.containerRefs.labelValueCountByLabelName}
        totalSeries={-1}
        tabId={"labelValueCountByLabelName"}
        sectionTitle={LABELS_CONTENT_TITLE}
        tableHeaderCells={LABEL_WITH_UNIQUE_VALUES_TABLE_HEADERS}
      />
    </>
  );
};

export default CardinalityPanel;
