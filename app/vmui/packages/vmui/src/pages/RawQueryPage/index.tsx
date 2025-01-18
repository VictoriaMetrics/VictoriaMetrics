import React, { FC, useState } from "preact/compat";
import LineLoader from "../../components/Main/LineLoader/LineLoader";
import { useCustomPanelState } from "../../state/customPanel/CustomPanelStateContext";
import { useQueryState } from "../../state/query/QueryStateContext";
import "../CustomPanel/style.scss";
import Alert from "../../components/Main/Alert/Alert";
import classNames from "classnames";
import useDeviceDetect from "../../hooks/useDeviceDetect";
import { useRef } from "react";
import CustomPanelTabs from "../CustomPanel/CustomPanelTabs";
import { DisplayTypeSwitch } from "../CustomPanel/DisplayTypeSwitch";
import QueryConfigurator from "../CustomPanel/QueryConfigurator/QueryConfigurator";
import WarningLimitSeries from "../CustomPanel/WarningLimitSeries/WarningLimitSeries";
import { useFetchExport } from "./hooks/useFetchExport";
import { useSetQueryParams } from "../CustomPanel/hooks/useSetQueryParams";
import { DisplayType } from "../../types";
import Hyperlink from "../../components/Main/Hyperlink/Hyperlink";
import { CloseIcon } from "../../components/Main/Icons";
import Button from "../../components/Main/Button/Button";
import DownloadReport, { ReportType } from "../CustomPanel/DownloadReport/DownloadReport";

const RawSamplesLink = () => (
  <Hyperlink
    href="https://docs.victoriametrics.com/keyconcepts/#raw-samples"
    underlined
  >
    raw samples
  </Hyperlink>
);

const QueryDataLink = () => (
  <Hyperlink
    underlined
    href="https://docs.victoriametrics.com/keyconcepts/#query-data"
  >
    Query API
  </Hyperlink>
);

const TimeSeriesSelectorLink = () => (
  <Hyperlink
    underlined
    href="https://docs.victoriametrics.com/keyconcepts/#filtering"
  >
    time series selector
  </Hyperlink>
);

const RawQueryPage: FC = () => {
  useSetQueryParams();
  const { isMobile } = useDeviceDetect();

  const { displayType } = useCustomPanelState();
  const { query } = useQueryState();

  const [hideQuery, setHideQuery] = useState<number[]>([]);
  const [hideError, setHideError] = useState(!query[0]);
  const [showAllSeries, setShowAllSeries] = useState(false);
  const [showPageDescription, setShowPageDescription] = useState(true);

  const {
    data,
    error,
    isLoading,
    warning,
    queryErrors,
    setQueryErrors,
    abortFetch,
    fetchUrl,
  } = useFetchExport({ hideQuery, showAllSeries });

  const controlsRef = useRef<HTMLDivElement>(null);

  const showError = !hideError && error;

  const handleHideQuery = (queries: number[]) => {
    setHideQuery(queries);
  };

  const handleRunQuery = () => {
    setHideError(false);
  };

  const handleHidePageDescription = () => {
    setShowPageDescription(false);
  };

  return (
    <div
      className={classNames({
        "vm-custom-panel": true,
        "vm-custom-panel_mobile": isMobile,
      })}
    >
      <QueryConfigurator
        label={"Time series selector"}
        queryErrors={!hideError ? queryErrors : []}
        setQueryErrors={setQueryErrors}
        setHideError={setHideError}
        stats={[]}
        isLoading={isLoading}
        onHideQuery={handleHideQuery}
        onRunQuery={handleRunQuery}
        abortFetch={abortFetch}
        hideButtons={{ traceQuery: true, disableCache: true }}
        includeFunctions={false}
      />
      {showPageDescription && (
        <Alert variant="info">
          <div className="vm-explore-metrics-header-description">
            <ul>
              <li>
                This page provides a dedicated view for querying and displaying <RawSamplesLink/> from VictoriaMetrics.
              </li>
              <li>
                It expects only <TimeSeriesSelectorLink/> as a query argument.
              </li>
              <li>
                Deduplication can only be disabled if it was previously enabled on the server
                (<code>-dedup.minScrapeInterval</code>).
              </li>
              <li>
                Users often assume that the <QueryDataLink/> returns data exactly as stored,
                but data samples and timestamps may be modified by the API.
              </li>
            </ul>
            <Button
              variant="text"
              size="small"
              startIcon={<CloseIcon/>}
              onClick={handleHidePageDescription}
              ariaLabel="close tips"
            />
          </div>
        </Alert>
      )}
      {showError && <Alert variant="error">{error}</Alert>}
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
        {isLoading && <LineLoader/>}
        <div
          className="vm-custom-panel-body-header"
          ref={controlsRef}
        >
          <div className="vm-custom-panel-body-header__tabs">
            <DisplayTypeSwitch tabFilter={(tab) => (tab.value !== DisplayType.table)}/>
          </div>
          {data && (
            <DownloadReport
              fetchUrl={fetchUrl}
              reportType={ReportType.RAW_DATA}
            />
          )}
        </div>
        <CustomPanelTabs
          graphData={data}
          liveData={data}
          isHistogram={false}
          displayType={displayType}
          controlsRef={controlsRef}
        />
      </div>
    </div>
  );
};

export default RawQueryPage;
