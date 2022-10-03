import React, {FC} from "preact/compat";
import {HashRouter, Route, Routes} from "react-router-dom";
import {SnackbarProvider} from "./contexts/Snackbar";
import {StateProvider} from "./state/common/StateContext";
import {AuthStateProvider} from "./state/auth/AuthStateContext";
import {GraphStateProvider} from "./state/graph/GraphStateContext";
import {CardinalityStateProvider} from "./state/cardinality/CardinalityStateContext";
import {TopQueriesStateProvider} from "./state/topQueries/TopQueriesStateContext";
import THEME from "./theme/theme";
import { ThemeProvider, StyledEngineProvider } from "@mui/material/styles";
import CssBaseline from "@mui/material/CssBaseline";
import LocalizationProvider from "@mui/lab/LocalizationProvider";
import DayjsUtils from "@date-io/dayjs";
import router from "./router/index";

import CustomPanel from "./components/CustomPanel/CustomPanel";
import HomeLayout from "./components/Home/HomeLayout";
import DashboardsLayout from "./components/PredefinedPanels/DashboardsLayout";
import CardinalityPanel from "./components/CardinalityPanel/CardinalityPanel";
import TopQueries from "./components/TopQueries/TopQueries";


const App: FC = () => {

  return <>
    <HashRouter>
      <CssBaseline /> {/* CSS Baseline: kind of normalize.css made by materialUI team - can be scoped */}
      <LocalizationProvider dateAdapter={DayjsUtils}> {/* Allows datepicker to work with DayJS */}
        <StyledEngineProvider injectFirst>
          <ThemeProvider theme={THEME}>  {/* Material UI theme customization */}
            <StateProvider> {/* Serialized into query string, common app settings */}
              <AuthStateProvider> {/* Auth related info - optionally persisted to Local Storage */}
                <GraphStateProvider> {/* Graph settings */}
                  <CardinalityStateProvider> {/* Cardinality settings */}
                    <TopQueriesStateProvider> {/* Top Queries settings */}
                      <SnackbarProvider> {/* Display various snackbars */}
                        <Routes>
                          <Route path={"/"} element={<HomeLayout/>}>
                            <Route path={router.home} element={<CustomPanel/>}/>
                            <Route path={router.dashboards} element={<DashboardsLayout/>}/>
                            <Route path={router.cardinality} element={<CardinalityPanel/>} />
                            <Route path={router.topQueries} element={<TopQueries/>} />
                          </Route>
                        </Routes>
                      </SnackbarProvider>
                    </TopQueriesStateProvider>
                  </CardinalityStateProvider>
                </GraphStateProvider>
              </AuthStateProvider>
            </StateProvider>
          </ThemeProvider>
        </StyledEngineProvider>
      </LocalizationProvider>
    </HashRouter>
  </>;
};

export default App;
