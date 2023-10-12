import React, { FC, useState, useEffect, useMemo } from "preact/compat";
import GraphView from "../../components/Views/GraphView/GraphView";
import QueryConfigurator from "./QueryConfigurator/QueryConfigurator";
import { useFetchQuery } from "../../hooks/useFetchQuery";
import JsonView from "../../components/Views/JsonView/JsonView";
import { DisplayTypeSwitch } from "./DisplayTypeSwitch";
import GraphSettings from "../../components/Configurators/GraphSettings/GraphSettings";
import { useGraphDispatch, useGraphState } from "../../state/graph/GraphStateContext";
import { AxisRange } from "../../state/graph/reducer";
import Spinner from "../../components/Main/Spinner/Spinner";
import TracingsView from "../../components/TraceQuery/TracingsView";
import Trace from "../../components/TraceQuery/Trace";
import TableSettings from "../../components/Table/TableSettings/TableSettings";
import { useCustomPanelState, useCustomPanelDispatch } from "../../state/customPanel/CustomPanelStateContext";
import { useQueryState } from "../../state/query/QueryStateContext";
import { useTimeDispatch, useTimeState } from "../../state/time/TimeStateContext";
import { useSetQueryParams } from "./hooks/useSetQueryParams";
import "./style.scss";
import Alert from "../../components/Main/Alert/Alert";
import TableView from "../../components/Views/TableView/TableView";
import Button from "../../components/Main/Button/Button";
import classNames from "classnames";
import useDeviceDetect from "../../hooks/useDeviceDetect";
import GraphTips from "../../components/Chart/GraphTips/GraphTips";
import InstantQueryTip from "./InstantQueryTip/InstantQueryTip";
import useBoolean from "../../hooks/useBoolean";
import { getColumns } from "../../hooks/useSortedCategories";
import useEventListener from "../../hooks/useEventListener";

const CustomPanel: FC = () => {
  const { displayType, isTracingEnabled } = useCustomPanelState();
  const { query } = useQueryState();
  const { period } = useTimeState();
  const timeDispatch = useTimeDispatch();
  const { isMobile } = useDeviceDetect();
  useSetQueryParams();

  const [displayColumns, setDisplayColumns] = useState<string[]>();
  const [tracesState, setTracesState] = useState<Trace[]>([]);
  const [hideQuery, setHideQuery] = useState<number[]>([]);
  const [hideError, setHideError] = useState(!query[0]);

  const {
    value: showAllSeries,
    setTrue: handleShowAll,
    setFalse: handleHideSeries,
  } = useBoolean(false);

  const { customStep, yaxis } = useGraphState();
  const graphDispatch = useGraphDispatch();

  const {
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

  const setYaxisLimits = (limits: AxisRange) => {
    graphDispatch({ type: "SET_YAXIS_LIMITS", payload: limits });
  };

  const toggleEnableLimits = () => {
    graphDispatch({ type: "TOGGLE_ENABLE_YAXIS_LIMITS" });
  };

  const setPeriod = ({ from, to }: {from: Date, to: Date}) => {
    timeDispatch({ type: "SET_PERIOD", payload: { from, to } });
  };

  const handleTraceDelete = (trace: Trace) => {
    const updatedTraces = tracesState.filter((data) => data.idValue !== trace.idValue);
    setTracesState([...updatedTraces]);
  };

  const handleHideQuery = (queries: number[]) => {
    setHideQuery(queries);
  };

  const handleRunQuery = () => {
    setHideError(false);
  };

  const columns = useMemo(() => getColumns(liveData || []).map(c => c.key), [liveData]);
  const { tableCompact } = useCustomPanelState();
  const customPanelDispatch = useCustomPanelDispatch();

  const toggleTableCompact = () => {
    customPanelDispatch({ type: "TOGGLE_TABLE_COMPACT" });
  };

  const handleChangePopstate = () => window.location.reload();
  useEventListener("popstate", handleChangePopstate);

  useEffect(() => {
    if (traces) {
      setTracesState([...tracesState, ...traces]);
    }
  }, [traces]);

  useEffect(() => {
    setTracesState([]);
  }, [displayType]);

  useEffect(handleHideSeries, [query]);

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
      {isTracingEnabled && (
        <div className="vm-custom-panel__trace">
          <TracingsView
            traces={tracesState}
            onDeleteClick={handleTraceDelete}
          />
        </div>
      )}
      {isLoading && <Spinner />}
      {!hideError && error && <Alert variant="error">{error}</Alert>}
      {!liveData?.length && (displayType !== "chart") && <Alert variant="info"><InstantQueryTip/></Alert>}
      {warning && <Alert variant="warning">
        <div
          className={classNames({
            "vm-custom-panel__warning": true,
            "vm-custom-panel__warning_mobile": isMobile
          })}
        >
          <p>{warning}</p>
          <Button
            color="warning"
            variant="outlined"
            onClick={handleShowAll}
          >
            Show all
          </Button>
        </div>
      </Alert>}
      <div
        className={classNames({
          "vm-custom-panel-body": true,
          "vm-custom-panel-body_mobile": isMobile,
          "vm-block": true,
          "vm-block_mobile": isMobile,
        })}
      >
        <div className="vm-custom-panel-body-header">
          <DisplayTypeSwitch/>
          {displayType === "chart" && (
            <div className="vm-custom-panel-body-header__left">
              <GraphTips/>
              <GraphSettings
                yaxis={yaxis}
                setYaxisLimits={setYaxisLimits}
                toggleEnableLimits={toggleEnableLimits}
              />
            </div>
          )}
          {displayType === "table" && (
            <TableSettings
              columns={columns}
              defaultColumns={displayColumns}
              onChangeColumns={setDisplayColumns}
              tableCompact={tableCompact}
              toggleTableCompact={toggleTableCompact}
            />
          )}
        </div>
        {graphData && period && (displayType === "chart") && (
          <GraphView
            data={graphData}
            period={period}
            customStep={customStep}
            query={query}
            yaxis={yaxis}
            setYaxisLimits={setYaxisLimits}
            setPeriod={setPeriod}
            height={isMobile ? window.innerHeight * 0.5 : 500}
            isHistogram={isHistogram}
          />
        )}
        {liveData && (displayType === "code") && (
          <JsonView data={liveData}/>
        )}
        {liveData && (displayType === "table") && (
          <TableView
            data={liveData}
            displayColumns={displayColumns}
          />
        )}
      </div>
    </div>
  );
};

export default CustomPanel;
