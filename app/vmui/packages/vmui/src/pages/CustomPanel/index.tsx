import React, { FC, useEffect, useState } from "preact/compat";
import QueryConfigurator from "./QueryConfigurator/QueryConfigurator";
import { useFetchQuery } from "../../hooks/useFetchQuery";
import { DisplayTypeSwitch } from "./DisplayTypeSwitch";
import { useGraphDispatch, useGraphState } from "../../state/graph/GraphStateContext";
import Spinner from "../../components/Main/Spinner/Spinner";
import { useCustomPanelState } from "../../state/customPanel/CustomPanelStateContext";
import { useQueryState } from "../../state/query/QueryStateContext";
import { useSetQueryParams } from "./hooks/useSetQueryParams";
import "./style.scss";
import Alert from "../../components/Main/Alert/Alert";
import classNames from "classnames";
import useDeviceDetect from "../../hooks/useDeviceDetect";
import InstantQueryTip from "./InstantQueryTip/InstantQueryTip";
import { useRef } from "react";
import CustomPanelTraces from "./CustomPanelTraces/CustomPanelTraces";
import WarningLimitSeries from "./WarningLimitSeries/WarningLimitSeries";
import CustomPanelTabs from "./CustomPanelTabs";
import { DisplayType } from "../../types";
import DownloadReport from "./DownloadReport/DownloadReport";

const CustomPanel: FC = () => {
  useSetQueryParams();
  const { isMobile } = useDeviceDetect();

  const { displayType } = useCustomPanelState();
  const { query } = useQueryState();
  const { customStep } = useGraphState();
  const graphDispatch = useGraphDispatch();

  const [hideQuery, setHideQuery] = useState<number[]>([]);
  const [hideError, setHideError] = useState(!query[0]);
  const [showAllSeries, setShowAllSeries] = useState(false);

  const controlsRef = useRef<HTMLDivElement>(null);

  const {
    fetchUrl,
    isLoading,
    liveData,
    graphData,
    error,
    queryErrors,
    setQueryErrors,
    queryStats,
    warning,
    traces,
    isHistogram
  } = useFetchQuery({
    visible: true,
    customStep,
    hideQuery,
    showAllSeries
  });

  const showInstantQueryTip = !liveData?.length && (displayType !== DisplayType.chart);
  const showError = !hideError && error;

  const handleHideQuery = (queries: number[]) => {
    setHideQuery(queries);
  };

  const handleRunQuery = () => {
    setHideError(false);
  };

  useEffect(() => {
    graphDispatch({ type: "SET_IS_HISTOGRAM", payload: isHistogram });
  }, [graphData]);

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
        onHideQuery={handleHideQuery}
        onRunQuery={handleRunQuery}
      />
      <CustomPanelTraces
        traces={traces}
        displayType={displayType}
      />
      {isLoading && <Spinner />}
      {showError && <Alert variant="error">{error}</Alert>}
      {showInstantQueryTip && <Alert variant="info"><InstantQueryTip/></Alert>}
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
          <div className="vm-custom-panel-body-header__tabs">
            <DisplayTypeSwitch/>
          </div>
          {(graphData || liveData) && <DownloadReport fetchUrl={fetchUrl}/>}
        </div>
        <CustomPanelTabs
          graphData={graphData}
          liveData={liveData}
          isHistogram={isHistogram}
          displayType={displayType}
          controlsRef={controlsRef}
        />
      </div>
    </div>
  );
};

export default CustomPanel;
