import React, { FC, useState } from "react";
import { SyntheticEvent } from "react";
import { useFetchQuery } from "./hooks/useCardinalityFetch";
import { queryUpdater } from "./helpers";
import { Data } from "../../components/Main/Table/types";
import CardinalityConfigurator from "./CardinalityConfigurator/CardinalityConfigurator";
import Spinner from "../../components/Main/Spinner/Spinner";
import { useCardinalityDispatch, useCardinalityState } from "../../state/cardinality/CardinalityStateContext";
import MetricsContent from "./MetricsContent/MetricsContent";
import { DefaultActiveTab, Tabs, TSDBStatus, Containers } from "./types";
import { useSetQueryParams } from "./hooks/useSetQueryParams";
import Alert from "../../components/Main/Alert/Alert";
import "./style.scss";

const spinnerMessage = `Please wait while cardinality stats is calculated. 
                        This may take some time if the db contains big number of time series.`;

const Index: FC = () => {
  const { topN, match, date, focusLabel } = useCardinalityState();
  const cardinalityDispatch = useCardinalityDispatch();
  useSetQueryParams();

  const configError = "";
  const [query, setQuery] = useState(match || "");
  const [queryHistoryIndex, setQueryHistoryIndex] = useState(0);
  const [queryHistory, setQueryHistory] = useState<string[]>([]);

  const onRunQuery = () => {
    setQueryHistory(prev => [...prev, query]);
    setQueryHistoryIndex(prev => prev + 1);
    cardinalityDispatch({ type: "SET_MATCH", payload: query });
    cardinalityDispatch({ type: "RUN_QUERY" });
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

  const onTopNChange = (value: string) => {
    cardinalityDispatch({ type: "SET_TOP_N", payload: +value });
  };

  const onFocusLabelChange = (value: string) => {
    cardinalityDispatch({ type: "SET_FOCUS_LABEL", payload: value });
  };

  const { isLoading, appConfigurator, error } = useFetchQuery();
  const [stateTabs, setTab] = useState(appConfigurator.defaultState.defaultActiveTab);
  const { tsdbStatusData, defaultState, tablesHeaders } = appConfigurator;
  const handleTabChange = (newValue: string, tabId: string) => {
    // eslint-disable-next-line @typescript-eslint/ban-ts-comment
    // @ts-ignore
    setTab({ ...stateTabs, [tabId]: +newValue });
  };

  const handleFilterClick = (key: string) => (e: SyntheticEvent) => {
    const name = e.currentTarget.id;
    const query = queryUpdater[key](focusLabel, name);
    setQuery(query);
    setQueryHistory(prev => [...prev, query]);
    setQueryHistoryIndex(prev => prev + 1);
    cardinalityDispatch({ type: "SET_MATCH", payload: query });
    let newFocusLabel = "";
    if (key === "labelValueCountByLabelName" || key == "seriesCountByLabelName") {
      newFocusLabel = name;
    }
    cardinalityDispatch({ type: "SET_FOCUS_LABEL", payload: newFocusLabel });
    cardinalityDispatch({ type: "RUN_QUERY" });
  };

  return (
    <div className="vm-cardinality-panel">
      {isLoading && <Spinner message={spinnerMessage}/>}
      <CardinalityConfigurator
        error={configError}
        query={query}
        topN={topN}
        date={date}
        match={match}
        totalSeries={tsdbStatusData.totalSeries}
        totalLabelValuePairs={tsdbStatusData.totalLabelValuePairs}
        focusLabel={focusLabel}
        onRunQuery={onRunQuery}
        onSetQuery={onSetQuery}
        onSetHistory={onSetHistory}
        onTopNChange={onTopNChange}
        onFocusLabelChange={onFocusLabelChange}
      />

      {error && <Alert variant="error">{error}</Alert>}

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
    </div>
  );
};

export default Index;
