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
import useSearchParamsFromObject from "../../hooks/useSearchParamsFromObject";

const spinnerMessage = `Please wait while cardinality stats is calculated. 
                        This may take some time if the db contains big number of time series.`;

const CardinalityPanel: FC = () => {
  const { isMobile } = useDeviceDetect();

  const [searchParams] = useSearchParams();
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();
  const showTips = searchParams.get("tips") || "";
  const match = searchParams.get("match") || "";
  const focusLabel = searchParams.get("focusLabel") || "";

  const { isLoading, appConfigurator, error, isCluster } = useFetchQuery();
  const { tsdbStatusData, getDefaultState, tablesHeaders, sectionsTips } = appConfigurator;
  const defaultState = getDefaultState(match, focusLabel);

  const handleFilterClick = (key: string) => (query: string) => {
    const value = queryUpdater[key]({ query, focusLabel, match });
    const params: Record<string, string> = { match: value };
    if (key === "labelValueCountByLabelName" || key == "seriesCountByLabelName") {
      params.focusLabel = query;
    }
    if (key == "seriesCountByFocusLabelValue") {
      params.focusLabel = "";
    }
    setSearchParamsFromKeys(params);
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
        isPrometheus={appConfigurator.isPrometheusData}
        totalSeries={tsdbStatusData.totalSeries}
        totalSeriesPrev={tsdbStatusData.totalSeriesPrev}
        totalSeriesAll={tsdbStatusData.totalSeriesByAll}
        totalLabelValuePairs={tsdbStatusData.totalLabelValuePairs}
        seriesCountByMetricName={tsdbStatusData.seriesCountByMetricName}
        isCluster={isCluster}
      />

      {showTips && (
        <div className="vm-cardinality-panel-tips">
          {!match && !focusLabel && <TipHighNumberOfSeries/>}
          {match && !focusLabel && <TipCardinalityOfSingle/>}
          {!match && !focusLabel && <TipHighNumberOfValues/>}
          {focusLabel && <TipCardinalityOfLabel />}
        </div>
      )}

      {error && <Alert variant="error">{error}</Alert>}

      {appConfigurator.keys(match, focusLabel).map((keyName) => {
        return <MetricsContent
          key={keyName}
          sectionTitle={appConfigurator.sectionsTitles(focusLabel)[keyName]}
          tip={sectionsTips[keyName]}
          rows={tsdbStatusData[keyName as keyof TSDBStatus] as unknown as Data[]}
          onActionClick={handleFilterClick(keyName)}
          tabs={defaultState.tabs[keyName as keyof Tabs]}
          chartContainer={defaultState.containerRefs[keyName as keyof Containers<HTMLDivElement>]}
          totalSeriesPrev={appConfigurator.totalSeries(keyName, true)}
          totalSeries={appConfigurator.totalSeries(keyName)}
          tableHeaderCells={tablesHeaders[keyName]}
          isPrometheus={appConfigurator.isPrometheusData}
        />;
      })}
    </div>
  );
};

export default CardinalityPanel;
