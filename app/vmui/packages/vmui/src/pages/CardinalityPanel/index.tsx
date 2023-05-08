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

const CardinalityPanel: FC = () => {
  const { isMobile } = useDeviceDetect();

  const [searchParams, setSearchParams] = useSearchParams();
  const showTips = searchParams.get("tips") || "";
  const match = searchParams.get("match") || "";
  const focusLabel = searchParams.get("focusLabel") || "";

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
        totalSeriesPrev={tsdbStatusData.totalSeriesPrev}
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

      {appConfigurator.keys(match, focusLabel).map((keyName) => {
        // do not use actions for 'labelValueCountByLabelName' when all filters are disabled
        const hasSetFields = !focusLabel && !match;
        const useActionForLabelValues = hasSetFields && keyName === "labelValueCountByLabelName";
        const action = !useActionForLabelValues ? handleFilterClick(keyName) : null;

        return <MetricsContent
          key={keyName}
          sectionTitle={appConfigurator.sectionsTitles(focusLabel)[keyName]}
          tip={sectionsTips[keyName]}
          rows={tsdbStatusData[keyName as keyof TSDBStatus] as unknown as Data[]}
          onActionClick={action}
          tabs={defaultState.tabs[keyName as keyof Tabs]}
          chartContainer={defaultState.containerRefs[keyName as keyof Containers<HTMLDivElement>]}
          totalSeriesPrev={appConfigurator.totalSeries(keyName, true)}
          totalSeries={appConfigurator.totalSeries(keyName)}
          tableHeaderCells={tablesHeaders[keyName]}
        />;
      })}
    </div>
  );
};

export default CardinalityPanel;
