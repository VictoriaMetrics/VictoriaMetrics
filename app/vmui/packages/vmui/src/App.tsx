import React, { FC } from "preact/compat";
import { HashRouter, Route, Routes } from "react-router-dom";
import THEME from "./theme/theme";
import { ThemeProvider, StyledEngineProvider } from "@mui/material/styles";
import CssBaseline from "@mui/material/CssBaseline";
import LocalizationProvider from "@mui/lab/LocalizationProvider";
import DayjsUtils from "@date-io/dayjs";
import router from "./router";
import CustomPanel from "./pages/CustomPanel/CustomPanel";
import HomeLayout from "./components/Home/HomeLayout";
import DashboardsLayout from "./pages/PredefinedPanels/DashboardLayout";
import CardinalityPanel from "./pages/CardinalityPanel/CardinalityPanel";
import TopQueries from "./pages/TopQueries/TopQueries";
import AppContextProvider from "./contexts/AppContextProvider";

const App: FC = () => {

  return <>
    <HashRouter>
      <CssBaseline/> {/* CSS Baseline: kind of normalize.css made by materialUI team - can be scoped */}
      <LocalizationProvider dateAdapter={DayjsUtils}> {/* Allows datepicker to work with DayJS */}
        <StyledEngineProvider injectFirst>
          <ThemeProvider theme={THEME}>  {/* Material UI theme customization */}
            <AppContextProvider>
              <Routes>
                <Route
                  path={"/"}
                  element={<HomeLayout/>}
                >
                  <Route
                    path={router.home}
                    element={<CustomPanel/>}
                  />
                  <Route
                    path={router.dashboards}
                    element={<DashboardsLayout/>}
                  />
                  <Route
                    path={router.cardinality}
                    element={<CardinalityPanel/>}
                  />
                  <Route
                    path={router.topQueries}
                    element={<TopQueries/>}
                  />
                </Route>
              </Routes>
            </AppContextProvider>
          </ThemeProvider>
        </StyledEngineProvider>
      </LocalizationProvider>
    </HashRouter>
  </>;
};

export default App;
