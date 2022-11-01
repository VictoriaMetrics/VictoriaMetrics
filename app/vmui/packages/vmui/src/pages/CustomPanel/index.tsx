import React, { FC, useState, useEffect } from "preact/compat";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import GraphView from "../../components/Views/GraphView";
import TableView from "../../components/Views/TableView";
import QueryConfigurator from "./QueryConfigurator";
import { useFetchQuery } from "../../hooks/useFetchQuery";
import JsonView from "../../components/Views/JsonView";
import { DisplayTypeSwitch } from "./DisplayTypeSwitch";
import GraphSettings from "../../components/Configurators/GraphSettings/GraphSettings";
import { useGraphDispatch, useGraphState } from "../../state/graph/GraphStateContext";
import { AxisRange } from "../../state/graph/reducer";
import Spinner from "../../components/Main/Spinner";
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
    <Box
      p={4}
      display="grid"
      gridTemplateRows="auto 1fr"
      style={{ minHeight: "calc(100vh - 64px)" }}
    >
      <Box
        boxShadow="rgba(99, 99, 99, 0.2) 0px 2px 8px 0px;"
        p={4}
        pb={2}
        m={-4}
        mb={2}
      >
        <QueryConfigurator
          error={error}
          queryOptions={queryOptions}
        />
      </Box>
      <Box height="100%">
        {isLoading && <Spinner
          isLoading={isLoading}
          height={"500px"}
        />}
        {<Box
          height={"100%"}
          bgcolor={"#fff"}
        >
          <Box
            display="grid"
            gridTemplateColumns="1fr auto"
            alignItems="center"
            mb={2}
            borderBottom={1}
            borderColor="divider"
          >
            <DisplayTypeSwitch/>
            <Box display={"flex"}>
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
            </Box>
          </Box>
          {error && <Alert
            color="error"
            severity="error"
            sx={{ whiteSpace: "pre-wrap", mt: 2 }}
          >{error}</Alert>}
          {warning && <Alert
            color="warning"
            severity="warning"
            sx={{ whiteSpace: "pre-wrap", my: 2 }}
          >{warning}</Alert>}
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
            <TableView
              data={liveData}
              displayColumns={displayColumns}
            />
          </>}
        </Box>}
      </Box>
    </Box>
  );
};

export default Index;
