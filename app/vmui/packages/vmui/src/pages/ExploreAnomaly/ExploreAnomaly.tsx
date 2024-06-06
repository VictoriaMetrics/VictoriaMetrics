import React, { FC, useMemo, useRef, useState } from "preact/compat";
import classNames from "classnames";
import useDeviceDetect from "../../hooks/useDeviceDetect";
import { ForecastType } from "../../types";
import { useSetQueryParams } from "../CustomPanel/hooks/useSetQueryParams";
import QueryConfigurator from "../CustomPanel/QueryConfigurator/QueryConfigurator";
import "../CustomPanel/style.scss";
import { useQueryState } from "../../state/query/QueryStateContext";
import { useFetchQuery } from "../../hooks/useFetchQuery";
import { useGraphState } from "../../state/graph/GraphStateContext";
import Spinner from "../../components/Main/Spinner/Spinner";
import Alert from "../../components/Main/Alert/Alert";
import WarningLimitSeries from "../CustomPanel/WarningLimitSeries/WarningLimitSeries";
import GraphTab from "../CustomPanel/CustomPanelTabs/GraphTab";
import { extractFields, isForecast } from "../../utils/uplot";
import { MetricResult } from "../../api/types";
import { promValueToNumber } from "../../utils/metric";

// Hardcoded to 1.0 for now; consider adding a UI slider for threshold adjustment in the future.
const ANOMALY_SCORE_THRESHOLD = 1;

const ExploreAnomaly: FC = () => {
  useSetQueryParams();
  const { isMobile } = useDeviceDetect();

  const { query } = useQueryState();
  const { customStep } = useGraphState();

  const controlsRef = useRef<HTMLDivElement>(null);

  const [hideQuery] = useState<number[]>([]);
  const [hideError, setHideError] = useState(!query[0]);
  const [showAllSeries, setShowAllSeries] = useState(false);

  const {
    isLoading,
    graphData,
    error,
    queryErrors,
    setQueryErrors,
    queryStats,
    warning,
  } = useFetchQuery({
    visible: true,
    customStep,
    hideQuery,
    showAllSeries
  });

  const data = useMemo(() => {
    if (!graphData) return [];
    const detectedData = graphData.map(d => ({ ...isForecast(d.metric), ...d }));
    const realData = detectedData.filter(d => d.value === ForecastType.actual);
    const anomalyScoreData = detectedData.filter(d => d.value === ForecastType.anomaly);
    const anomalyData: MetricResult[] = realData.map((d) => {
      const id = extractFields(d.metric);
      const anomalyScoreDataByLabels = anomalyScoreData.find(du => extractFields(du.metric) === id);

      return {
        group: 1,
        metric: { ...d.metric, __name__: ForecastType.anomaly },
        values: d.values.filter(([t]) => {
          if (!anomalyScoreDataByLabels) return false;
          const anomalyScore = anomalyScoreDataByLabels.values.find(([tMax]) => tMax === t) as [number, string];
          return anomalyScore && promValueToNumber(anomalyScore[1]) > ANOMALY_SCORE_THRESHOLD;
        })
      };
    });
    const filterData = detectedData.filter(d => (d.value !== ForecastType.anomaly) && d.value) as MetricResult[];
    return filterData.concat(anomalyData);
  }, [graphData]);

  const handleRunQuery = () => {
    setHideError(false);
  };

  return (
    <div
      className={classNames({
        "vm-custom-panel": true,
        "vm-custom-panel_mobile": isMobile,
      })}
    >
      <QueryConfigurator
        queryErrors={!hideError ? queryErrors : []}
        setQueryErrors={setQueryErrors}
        setHideError={setHideError}
        stats={queryStats}
        onRunQuery={handleRunQuery}
        hideButtons={{ addQuery: true, prettify: false, autocomplete: false, traceQuery: true, anomalyConfig: true }}
      />
      {isLoading && <Spinner/>}
      {(!hideError && error) && <Alert variant="error">{error}</Alert>}
      {warning && (
        <WarningLimitSeries
          warning={warning}
          query={query}
          onChange={setShowAllSeries}
        />
      )}
      <div
        className={classNames({
          "vm-custom-panel-body": true,
          "vm-custom-panel-body_mobile": isMobile,
          "vm-block": true,
          "vm-block_mobile": isMobile,
        })}
      >
        <div
          className="vm-custom-panel-body-header"
          ref={controlsRef}
        >
          <div/>
        </div>
        {data && (
          <GraphTab
            graphData={data}
            isHistogram={false}
            controlsRef={controlsRef}
            isAnomalyView={true}
          />
        )}
      </div>
    </div>
  );
};

export default ExploreAnomaly;
