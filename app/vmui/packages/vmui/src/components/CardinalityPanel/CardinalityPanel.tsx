import React, {ChangeEvent, FC, useState} from "react";
import {SyntheticEvent} from "react";
import {Alert} from "@mui/material";
import {useFetchQuery} from "../../hooks/useCardinalityFetch";
import {queryUpdater} from "./helpers";
import {Data} from "../Table/types";
import CardinalityConfigurator from "./CardinalityConfigurator/CardinalityConfigurator";
import Spinner from "../common/Spinner";
import {useCardinalityDispatch, useCardinalityState} from "../../state/cardinality/CardinalityStateContext";
import MetricsContent from "./MetricsContent/MetricsContent";
import {DefaultActiveTab, Tabs, TSDBStatus, Containers} from "./types";

const spinnerContainerStyles = (height: string) =>  {
  return {
    width: "100%",
    maxWidth: "100%",
    position: "absolute",
    height: height ?? "50%",
    background: "rgba(255, 255, 255, 0.7)",
    pointerEvents: "none",
    zIndex: 1000,
  };
};

const CardinalityPanel: FC = () => {
  const cardinalityDispatch = useCardinalityDispatch();

  const {topN, match, date, focusLabel} = useCardinalityState();
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

  const onFocusLabelChange = (e: ChangeEvent<HTMLTextAreaElement|HTMLInputElement>) => {
    cardinalityDispatch({type: "SET_FOCUS_LABEL", payload: e.target.value});
  };

  const {isLoading, appConfigurator, error} = useFetchQuery();
  const [stateTabs, setTab] = useState(appConfigurator.defaultState.defaultActiveTab);
  const {tsdbStatusData, defaultState, tablesHeaders} = appConfigurator;
  const handleTabChange = (e: SyntheticEvent, newValue: number) => {
    // eslint-disable-next-line @typescript-eslint/ban-ts-comment
    // @ts-ignore
    setTab({...stateTabs, [e.target.id]: newValue});
  };

  const handleFilterClick = (key: string) => (e: SyntheticEvent) => {
    const name = e.currentTarget.id;
    const query = queryUpdater[key](focusLabel, name);
    setQuery(query);
    setQueryHistory(prev => [...prev, query]);
    setQueryHistoryIndex(prev => prev + 1);
    cardinalityDispatch({type: "SET_MATCH", payload: query});
    let newFocusLabel = "";
    if (key === "labelValueCountByLabelName" || key == "seriesCountByLabelName") {
      newFocusLabel = name;
    }
    cardinalityDispatch({type: "SET_FOCUS_LABEL", payload: newFocusLabel});
    cardinalityDispatch({type: "RUN_QUERY"});
  };

  return (
    <>
      {isLoading && <Spinner
        isLoading={isLoading}
        height={"800px"}
        containerStyles={spinnerContainerStyles("100%")}
        title={<Alert color="error" severity="error" sx={{whiteSpace: "pre-wrap", mt: 2}}>
          Please wait while cardinality stats is calculated. This may take some time if the db contains big number of time series
        </Alert>}
      />}
      <CardinalityConfigurator error={configError} query={query} onRunQuery={onRunQuery} onSetQuery={onSetQuery}
        onSetHistory={onSetHistory} onTopNChange={onTopNChange} topN={topN} date={date} match={match}
        totalSeries={tsdbStatusData.totalSeries} totalLabelValuePairs={tsdbStatusData.totalLabelValuePairs}
        focusLabel={focusLabel} onFocusLabelChange={onFocusLabelChange}
      />
      {error && <Alert color="error" severity="error" sx={{whiteSpace: "pre-wrap", mt: 2}}>{error}</Alert>}
      {appConfigurator.keys(focusLabel).map((keyName) => (
        <MetricsContent
          key={keyName}
          sectionTitle={appConfigurator.sectionsTitles(focusLabel)[keyName]}
          activeTab={stateTabs[keyName as keyof DefaultActiveTab]}
          rows={tsdbStatusData[keyName as keyof TSDBStatus] as unknown as Data[]}
          onChange={handleTabChange}
          onActionClick={handleFilterClick(keyName)}
          tabs={defaultState.tabs[keyName as keyof Tabs]}
          chartContainer={defaultState.containerRefs[keyName as keyof Containers<HTMLDivElement>]}
          totalSeries={appConfigurator.totalSeries(keyName)}
          tabId={keyName}
          tableHeaderCells={tablesHeaders[keyName]}
        />
      ))}
    </>
  );
};

export default CardinalityPanel;
