import React, {FC} from "preact/compat";
import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import CircularProgress from "@mui/material/CircularProgress";
import Fade from "@mui/material/Fade";
import GraphView from "./Views/GraphView";
import TableView from "./Views/TableView";
import {useAppState} from "../../state/common/StateContext";
import QueryConfigurator from "./Configurator/Query/QueryConfigurator";
import {useFetchQuery} from "./Configurator/Query/useFetchQuery";
import JsonView from "./Views/JsonView";
import Header from "../Header/Header";
import {DisplayTypeSwitch} from "./Configurator/DisplayTypeSwitch";
import GraphSettings from "./Configurator/Graph/GraphSettings";

const HomeLayout: FC = () => {

  const {displayType, time: {period}} = useAppState();

  const {isLoading, liveData, graphData, error, queryOptions} = useFetchQuery();

  return (
    <Box id="homeLayout">
      <Header/>
      <Box p={4} display="grid" gridTemplateRows="auto 1fr" style={{minHeight: "calc(100vh - 64px)"}}>
        <QueryConfigurator error={error} queryOptions={queryOptions}/>
        <Box height="100%">
          {isLoading && <Fade in={isLoading} style={{
            transitionDelay: isLoading ? "300ms" : "0ms",
          }}>
            <Box alignItems="center" justifyContent="center" flexDirection="column" display="flex"
              style={{
                width: "100%",
                maxWidth: "calc(100vw - 64px)",
                position: "absolute",
                height: "50%",
                background: "linear-gradient(rgba(255,255,255,.7), rgba(255,255,255,.7), rgba(255,255,255,0))"
              }}>
              <CircularProgress/>
            </Box>
          </Fade>}
          {<Box height={"100%"} bgcolor={"#fff"}>
            <Box display="grid" gridTemplateColumns="1fr auto" alignItems="center" mx={-4} px={4} mb={2}
              borderBottom={1} borderColor="divider">
              <DisplayTypeSwitch/>
              {displayType === "chart" &&  <GraphSettings/>}
            </Box>
            {error && <Alert color="error" severity="error"
              style={{fontSize: "14px", whiteSpace: "pre-wrap", marginTop: "20px"}}>
              {error}
            </Alert>}
            {graphData && period && (displayType === "chart") && <GraphView data={graphData}/>}
            {liveData && (displayType === "code") && <JsonView data={liveData}/>}
            {liveData && (displayType === "table") && <TableView data={liveData}/>}
          </Box>}
        </Box>
      </Box>
    </Box>
  );
};

export default HomeLayout;