import React, {FC} from "preact/compat";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import GraphView from "./Views/GraphView";
import TableView from "./Views/TableView";
import {useAppDispatch, useAppState} from "../../state/common/StateContext";
import QueryConfigurator from "./Configurator/Query/QueryConfigurator";
import {useFetchQuery} from "../../hooks/useFetchQuery";
import JsonView from "./Views/JsonView";
import {DisplayTypeSwitch} from "./Configurator/DisplayTypeSwitch";
import GraphSettings from "./Configurator/Graph/GraphSettings";
import {useGraphDispatch, useGraphState} from "../../state/graph/GraphStateContext";
import {AxisRange} from "../../state/graph/reducer";
import Spinner from "../common/Spinner";

const CustomPanel: FC = () => {

  const {displayType, time: {period}, query} = useAppState();
  const { customStep, yaxis } = useGraphState();

  const dispatch = useAppDispatch();
  const graphDispatch = useGraphDispatch();

  const setYaxisLimits = (limits: AxisRange) => {
    graphDispatch({type: "SET_YAXIS_LIMITS", payload: limits});
  };

  const toggleEnableLimits = () => {
    graphDispatch({type: "TOGGLE_ENABLE_YAXIS_LIMITS"});
  };

  const setPeriod = ({from, to}: {from: Date, to: Date}) => {
    dispatch({type: "SET_PERIOD", payload: {from, to}});
  };

  const {isLoading, liveData, graphData, error, queryOptions} = useFetchQuery({
    visible: true,
    customStep
  });

  return (
    <Box p={4} display="grid" gridTemplateRows="auto 1fr" style={{minHeight: "calc(100vh - 64px)"}}>
      <QueryConfigurator error={error} queryOptions={queryOptions}/>
      <Box height="100%">
        {isLoading && <Spinner isLoading={isLoading} height={"500px"}/>}
        {<Box height={"100%"} bgcolor={"#fff"}>
          <Box display="grid" gridTemplateColumns="1fr auto" alignItems="center" mx={-4} px={4} mb={2}
            borderBottom={1} borderColor="divider">
            <DisplayTypeSwitch/>
            {displayType === "chart" && <GraphSettings
              yaxis={yaxis}
              setYaxisLimits={setYaxisLimits}
              toggleEnableLimits={toggleEnableLimits}
            />}
          </Box>
          {error && <Alert color="error" severity="error" sx={{whiteSpace: "pre-wrap", mt: 2}}>{error}</Alert>}
          {graphData && period && (displayType === "chart") &&
            <GraphView data={graphData} period={period} customStep={customStep} query={query} yaxis={yaxis}
              setYaxisLimits={setYaxisLimits} setPeriod={setPeriod}/>}
          {liveData && (displayType === "code") && <JsonView data={liveData}/>}
          {liveData && (displayType === "table") && <TableView data={liveData}/>}
        </Box>}
      </Box>
    </Box>
  );
};

export default CustomPanel;
