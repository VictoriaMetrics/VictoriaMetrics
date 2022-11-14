import React, { FC, useState, useEffect } from "preact/compat";
import GraphView from "../../components/Views/GraphView/GraphView";
import QueryConfigurator from "./QueryConfigurator/QueryConfigurator";
import { useFetchQuery } from "../../hooks/useFetchQuery";
import JsonView from "../../components/Views/JsonView/JsonView";
import { DisplayTypeSwitch } from "./DisplayTypeSwitch";
import GraphSettings from "../../components/Configurators/GraphSettings/GraphSettings";
import { useGraphDispatch, useGraphState } from "../../state/graph/GraphStateContext";
import { AxisRange } from "../../state/graph/reducer";
import Spinner from "../../components/Main/Spinner/Spinner";
import { useFetchQueryOptions } from "../../hooks/useFetchQueryOptions";
import TracingsView from "../../components/TraceQuery/TracingsView";
import Trace from "../../components/TraceQuery/Trace";
import TableSettings from "../CardinalityPanel/Table/TableSettings/TableSettings";
import { useCustomPanelState } from "../../state/customPanel/CustomPanelStateContext";
import { useQueryState } from "../../state/query/QueryStateContext";
import { useTimeDispatch, useTimeState } from "../../state/time/TimeStateContext";
import { useSetQueryParams } from "./hooks/useSetQueryParams";
import "./style.scss";
import Alert from "../../components/Main/Alert/Alert";
import TableView from "../../components/Views/TableView/TableView";

const CustomPanel: FC = () => {
  const { displayType, isTracingEnabled } = useCustomPanelState();
  const { query } = useQueryState();
  const { period } = useTimeState();
  const timeDispatch = useTimeDispatch();
  useSetQueryParams();

  const [displayColumns, setDisplayColumns] = useState<string[]>();
  const [tracesState, setTracesState] = useState<Trace[]>([]);

  const { customStep, yaxis } = useGraphState();
  const graphDispatch = useGraphDispatch();

  const setYaxisLimits = (limits: AxisRange) => {
    graphDispatch({ type: "SET_YAXIS_LIMITS", payload: limits });
  };

  const toggleEnableLimits = () => {
    graphDispatch({ type: "TOGGLE_ENABLE_YAXIS_LIMITS" });
  };

  const setPeriod = ({ from, to }: {from: Date, to: Date}) => {
    timeDispatch({ type: "SET_PERIOD", payload: { from, to } });
  };

  const { queryOptions } = useFetchQueryOptions();
  const { isLoading, liveData, graphData, error, warning, traces } = useFetchQuery({
    visible: true,
    customStep
  });

  const handleTraceDelete = (trace: Trace) => {
    const updatedTraces = tracesState.filter((data) => data.idValue !== trace.idValue);
    setTracesState([...updatedTraces]);
  };

  useEffect(() => {
    if (traces) {
      setTracesState([...tracesState, ...traces]);
    }
  }, [traces]);

  useEffect(() => {
    setTracesState([]);
  }, [displayType]);

  return (
    <div className="vm-custom-panel">
      <QueryConfigurator
        error={error}
        queryOptions={queryOptions}
      />
      {isTracingEnabled && (
        <div className="vm-custom-panel__trace">
          <TracingsView
            traces={tracesState}
            onDeleteClick={handleTraceDelete}
          />
        </div>
      )}
      {error && <Alert variant="error">{error}</Alert>}
      {warning && <Alert variant="warning">{warning}</Alert>}
      <div className="vm-custom-panel-body vm-block">
        {isLoading && <Spinner />}
        <div className="vm-custom-panel-body-header">
          <DisplayTypeSwitch/>
          {displayType === "chart" && <GraphSettings
            yaxis={yaxis}
            setYaxisLimits={setYaxisLimits}
            toggleEnableLimits={toggleEnableLimits}
          />}
          {displayType === "table" && <TableSettings
            data={liveData || []}
            defaultColumns={displayColumns}
            onChange={setDisplayColumns}
          />}
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
          />
        )}
        {liveData && (displayType === "code") && <JsonView data={liveData}/>}
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
