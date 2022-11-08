import React, { FC, useState, useEffect } from "preact/compat";
import GraphView from "../../components/Views/GraphView";
// import TableView from "../../components/Views/TableView";
import QueryConfigurator from "./QueryConfigurator";
import { useFetchQuery } from "../../hooks/useFetchQuery";
import JsonView from "../../components/Views/JsonView";
import { DisplayTypeSwitch } from "./DisplayTypeSwitch";
import GraphSettings from "../../components/Configurators/GraphSettings/GraphSettings";
import { useGraphDispatch, useGraphState } from "../../state/graph/GraphStateContext";
import { AxisRange } from "../../state/graph/reducer";
import Spinner from "../../components/Main/Spinner/Spinner";
import { useFetchQueryOptions } from "../../hooks/useFetchQueryOptions";
import TraceQuery from "../../components/TraceQuery/TracingsView";
import Trace from "../../components/TraceQuery/Trace";
import TableSettings from "../../components/Main/Table/TableSettings";
import { useCustomPanelState } from "../../state/customPanel/CustomPanelStateContext";
import { useQueryState } from "../../state/query/QueryStateContext";
import { useTimeDispatch, useTimeState } from "../../state/time/TimeStateContext";
import { useSetQueryParams } from "./hooks/useSetQueryParams";

const Index: FC = () => {
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
    <div>
      <div>
        <QueryConfigurator
          error={error}
          queryOptions={queryOptions}
        />
      </div>
      <div>
        {isLoading && <Spinner containerStyles={{ height: "500px" }}/>}
        {<div>
          <div>
            <DisplayTypeSwitch/>
            <div>
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
          </div>
          {/*{error && <Alert*/}
          {/*  color="error"*/}
          {/*  severity="error"*/}
          {/*  sx={{ whiteSpace: "pre-wrap", mt: 2 }}*/}
          {/*>{error}</Alert>}*/}
          {/*{warning && <Alert*/}
          {/*  color="warning"*/}
          {/*  severity="warning"*/}
          {/*  sx={{ whiteSpace: "pre-wrap", my: 2 }}*/}
          {/*>{warning}</Alert>}*/}
          {graphData && period && (displayType === "chart") && <>
            {isTracingEnabled && <TraceQuery
              traces={tracesState}
              onDeleteClick={handleTraceDelete}
            />}
            <GraphView
              data={graphData}
              period={period}
              customStep={customStep}
              query={query}
              yaxis={yaxis}
              setYaxisLimits={setYaxisLimits}
              setPeriod={setPeriod}
            />
          </>}
          {liveData && (displayType === "code") && <JsonView data={liveData}/>}
          {liveData && (displayType === "table") && <>
            {isTracingEnabled && <TraceQuery
              traces={tracesState}
              onDeleteClick={handleTraceDelete}
            />}
            coming soon
            {/*<TableView*/}
            {/*  data={liveData}*/}
            {/*  displayColumns={displayColumns}*/}
            {/*/>*/}
          </>}
        </div>}
      </div>
    </div>
  );
};

export default Index;
