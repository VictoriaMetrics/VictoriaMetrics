import { FC, useEffect, useState, useMemo, useRef, useCallback } from "preact/compat";
import QueryConfigurator from "./QueryConfigurator/QueryConfigurator";
import { useFetchQuery } from "../../hooks/useFetchQuery";
import { DisplayTypeSwitch } from "./DisplayTypeSwitch";
import { useGraphDispatch, useGraphState } from "../../state/graph/GraphStateContext";
import LineLoader from "../../components/Main/LineLoader/LineLoader";
import { useCustomPanelState } from "../../state/customPanel/CustomPanelStateContext";
import { useQueryState } from "../../state/query/QueryStateContext";
import { useSetQueryParams } from "./hooks/useSetQueryParams";
import "./style.scss";
import Alert from "../../components/Main/Alert/Alert";
import classNames from "classnames";
import useDeviceDetect from "../../hooks/useDeviceDetect";
import InstantQueryTip from "./InstantQueryTip/InstantQueryTip";
import CustomPanelTraces from "./CustomPanelTraces/CustomPanelTraces";
import WarningLimitSeries from "./WarningLimitSeries/WarningLimitSeries";
import CustomPanelTabs from "./CustomPanelTabs";
import { DisplayType } from "../../types";
import DownloadReport from "./DownloadReport/DownloadReport";
import WarningHeatmapToLine from "./WarningHeatmapToLine/WarningHeatmapToLine";
import DownloadButton from "../../components/DownloadButton/DownloadButton";
import { downloadCSV, downloadJSON } from "../../utils/file";
import { convertMetricsDataToCSV } from "./utils";

type ExportFormats = "csv" | "json";

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
    isHistogram,
    abortFetch,
  } = useFetchQuery({
    visible: true,
    customStep,
    hideQuery,
    showAllSeries
  });

  const fileDownloaders = useMemo(() => {
    const getFilename = (format: ExportFormats) => {
      return `vmui_export_${query.join("_")}.${format}`;
    };

    return {
      csv: async () => {
        if(!liveData) return;
        const csvData = convertMetricsDataToCSV(liveData);
        downloadCSV(csvData, getFilename("csv"));
      },
      json: async () => {
        downloadJSON(JSON.stringify(liveData), getFilename("json"));
      },
    };
  }, [liveData, query]);

  const onDownloadClick = useCallback((format?: ExportFormats) => {
    format && fileDownloaders[format]();
  }, [fileDownloaders]);

  const showInstantQueryTip = !liveData?.length && (displayType !== DisplayType.chart);
  const showError = !hideError && error;

  const handleHideQuery = useCallback((queries: number[]) => {
    setHideQuery(queries);
  }, []);

  const handleRunQuery = useCallback(() => {
    setHideError(false);
  }, []);

  useEffect(() => {
    graphDispatch({ type: "SET_IS_HISTOGRAM", payload: isHistogram });
  }, [isHistogram]);

  return (
    <div
      className={classNames({
        "vm-custom-panel": true,
        "vm-custom-panel_mobile": isMobile,
      })}
    >
      <QueryConfigurator
        queryErrors={!hideError ? queryErrors : undefined}
        setQueryErrors={setQueryErrors}
        setHideError={setHideError}
        stats={queryStats}
        isLoading={isLoading}
        onHideQuery={handleHideQuery}
        onRunQuery={handleRunQuery}
        abortFetch={abortFetch}
        hideButtons={{ reduceMemUsage: true }}
      />
      <CustomPanelTraces
        traces={traces}
        displayType={displayType}
      />
      {showError && <Alert variant="error">{error}</Alert>}
      {showInstantQueryTip && <Alert variant="info"><InstantQueryTip/></Alert>}
      <WarningHeatmapToLine/>
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
            <DisplayTypeSwitch/>
          </div>
          {displayType === "table" && (
            <DownloadButton
              title={"Export query"}
              onDownload={onDownloadClick}
              downloadFormatOptions={["json", "csv"]}
            />)}
          {(graphData || liveData) && displayType !== "code" && <DownloadReport fetchUrl={fetchUrl}/>}
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
