import React, {FC} from "react";
import {AppBar, Box, CircularProgress, Fade, Link, Toolbar, Typography} from "@material-ui/core";
import {ExecutionControls} from "./Configurator/ExecutionControls";
import {DisplayTypeSwitch} from "./Configurator/DisplayTypeSwitch";
import GraphView from "./Views/GraphView";
import TableView from "./Views/TableView";
import {useAppState} from "../../state/common/StateContext";
import QueryConfigurator from "./Configurator/QueryConfigurator";
import {useFetchQuery} from "./Configurator/useFetchQuery";
import JsonView from "./Views/JsonView";
import {UrlCopy} from "./UrlCopy";
import {Alert} from "@material-ui/lab";

const HomeLayout: FC = () => {

  const {displayType, time: {period}} = useAppState();

  const {fetchUrl, isLoading, liveData, graphData, error} = useFetchQuery();

  return (
    <>
      <AppBar position="static">
        <Toolbar>
          <Box mr={2} display="flex">
            <Typography variant="h5">
              <span style={{fontWeight: "bolder"}}>VM</span>
              <span style={{fontWeight: "lighter"}}>UI</span>
            </Typography>
            <div style={{
              fontSize: "10px",
              marginTop: "-2px"
            }}>
              <div>BETA</div>
            </div>
          </Box>
          <div style={{
            fontSize: "10px",
            position: "absolute",
            top: "40px",
            opacity: ".4"
          }}>
            <Link color="inherit" href="https://github.com/VictoriaMetrics/vmui/issues/new" target="_blank">
              Create an issue
            </Link>
          </div>
          <Box flexGrow={1}>
            <ExecutionControls/>
          </Box>
          <DisplayTypeSwitch/>
          <UrlCopy url={fetchUrl}/>
        </Toolbar>
      </AppBar>
      <Box display="flex" flexDirection="column" style={{height: "calc(100vh - 64px)"}}>
        <Box m={2}>
          <QueryConfigurator/>
        </Box>
        <Box flexShrink={1} style={{overflowY: "scroll"}}>
          {isLoading && <Fade in={isLoading} style={{
            transitionDelay: isLoading ? "300ms" : "0ms",
          }}>
            <Box alignItems="center" flexDirection="column" display="flex"
              style={{
                width: "100%",
                position: "absolute",
                height: "150px",
                background: "linear-gradient(rgba(255,255,255,.7), rgba(255,255,255,.7), rgba(255,255,255,0))"
              }} m={2}>
              <CircularProgress/>
            </Box>
          </Fade>}
          {<Box p={2}>
            {error &&
            <Alert color="error" style={{fontSize: "14px"}}>
              {error}
            </Alert>}
            {graphData && period && (displayType === "chart") &&
              <GraphView data={graphData} timePresets={period}></GraphView>}
            {liveData && (displayType === "code") && <JsonView data={liveData}/>}
            {liveData && (displayType === "table") && <TableView data={liveData}/>}
          </Box>}
        </Box>
      </Box>
    </>
  );
};

export default HomeLayout;