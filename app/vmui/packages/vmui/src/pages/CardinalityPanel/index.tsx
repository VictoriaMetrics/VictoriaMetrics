import React, { FC } from "react";
import { useFetchQuery } from "./hooks/useCardinalityFetch";
import { queryUpdater } from "./helpers";
import { Data } from "./Table/types";
import CardinalityConfigurator from "./CardinalityConfigurator/CardinalityConfigurator";
import Spinner from "../../components/Main/Spinner/Spinner";
import MetricsContent from "./MetricsContent/MetricsContent";
import { Tabs, TSDBStatus, Containers } from "./types";
import Alert from "../../components/Main/Alert/Alert";
import "./style.scss";
import classNames from "classnames";
import useDeviceDetect from "../../hooks/useDeviceDetect";
import { useSearchParams } from "react-router-dom";
import {
  TipCardinalityOfLabel,
  TipCardinalityOfSingle,
  TipHighNumberOfSeries,
  TipHighNumberOfValues
} from "./CardinalityTips";

const spinnerMessage = `Please wait while cardinality stats is calculated. 
                        This may take some time if the db contains big number of time series.`;

const Index: FC = () => {
  const { isMobile } = useDeviceDetect();

  const [searchParams, setSearchParams] = useSearchParams();
  const showTips = searchParams.get("tips") || "";
  const match = searchParams.get("match") || "";
  const focusLabel = searchParams.get("focusLabel") || "";
  const isMetric = match && /__name__=".+"/.test(match);

  const { isLoading, appConfigurator, error } = useFetchQuery();
  const { tsdbStatusData, getDefaultState, tablesHeaders, sectionsTips } = appConfigurator;
  const defaultState = getDefaultState(match, focusLabel);

  const handleFilterClick = (key: string) => (query: string) => {
    const value = queryUpdater[key]({ query, focusLabel, match });
    searchParams.set("match", value);
    if (key === "labelValueCountByLabelName" || key == "seriesCountByLabelName") {
      searchParams.set("focusLabel", query);
    }
    if (key == "seriesCountByFocusLabelValue") {
      searchParams.set("focusLabel", "");
    }
    setSearchParams(searchParams);
  };

  return (
    <div
      className={classNames({
        "vm-cardinality-panel": true,
        "vm-cardinality-panel_mobile": isMobile
      })}
    >
      {isLoading && <Spinner message={spinnerMessage}/>}
      <CardinalityConfigurator
        totalSeries={tsdbStatusData.totalSeries}
        totalSeriesAll={tsdbStatusData.totalSeriesByAll}
        totalLabelValuePairs={tsdbStatusData.totalLabelValuePairs}
        seriesCountByMetricName={tsdbStatusData.seriesCountByMetricName}
      />

      {showTips && (
        <div className="vm-cardinality-panel-tips">
          {!match && !focusLabel && <TipHighNumberOfSeries/>}
          {match && !focusLabel && <TipCardinalityOfSingle/>}
          {!match && !focusLabel && <TipHighNumberOfValues/>}
          {focusLabel && <TipCardinalityOfLabel/>}
        </div>
      )}

      {error && <Alert variant="error">{error}</Alert>}

      {appConfigurator.keys(match, focusLabel).map((keyName) => (
        <MetricsContent
          key={keyName}
          sectionTitle={appConfigurator.sectionsTitles(focusLabel)[keyName]}
          tip={sectionsTips[keyName]}
          rows={tsdbStatusData[keyName as keyof TSDBStatus] as unknown as Data[]}
          onActionClick={handleFilterClick(keyName)}
          tabs={defaultState.tabs[keyName as keyof Tabs]}
          chartContainer={defaultState.containerRefs[keyName as keyof Containers<HTMLDivElement>]}
          totalSeries={appConfigurator.totalSeries(keyName)}
          tableHeaderCells={tablesHeaders[keyName]}
        />
      ))}
      {isMetric && !focusLabel && (
        <div
          className={classNames({
            "vm-block": true,
            "vm-block_mobile": isMobile
          })}
        >
          <div className="vm-metrics-content-header vm-section-header vm-cardinality-panel-pairs__header">
            <h5
              className={classNames({
                "vm-metrics-content-header__title": true,
                "vm-section-header__title": true,
                "vm-section-header__title_mobile": isMobile,
              })}
            >
              Label=value pairs with the highest number of series
            </h5>
          </div>
          <div className="vm-cardinality-panel-pairs">
            {tsdbStatusData.seriesCountByLabelValuePair.map((item) => (
              <div
                className="vm-cardinality-panel-pairs__item"
                key={item.name}
              >
                {item.name}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
};

export default Index;
