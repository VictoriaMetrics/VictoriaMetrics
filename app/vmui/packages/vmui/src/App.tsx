import React, {FC} from "preact/compat";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import {SnackbarProvider} from "./contexts/Snackbar";
import {StateProvider} from "./state/common/StateContext";
import {AuthStateProvider} from "./state/auth/AuthStateContext";
import {GraphStateProvider} from "./state/graph/GraphStateContext";
import THEME from "./theme/theme";
import { ThemeProvider, StyledEngineProvider } from "@mui/material/styles";
import CssBaseline from "@mui/material/CssBaseline";
import LocalizationProvider from "@mui/lab/LocalizationProvider";
import DayjsUtils from "@date-io/dayjs";
import router from "./router/index";

import HomeLayout from "./components/Home/HomeLayout";
import DashboardsLayout from "./components/PredefinedPanels/DashboardsLayout";

const [baseRoute] = window.location.href.match(/\/(?:graph|vmui)/) || ["/"];

const App: FC = () => {

  return <>
    <CssBaseline /> {/* CSS Baseline: kind of normalize.css made by materialUI team - can be scoped */}
    <LocalizationProvider dateAdapter={DayjsUtils}> {/* Allows datepicker to work with DayJS */}
      <StyledEngineProvider injectFirst>
        <ThemeProvider theme={THEME}>  {/* Material UI theme customization */}
          <StateProvider> {/* Serialized into query string, common app settings */}
            <AuthStateProvider> {/* Auth related info - optionally persisted to Local Storage */}
              <GraphStateProvider> {/* Graph settings */}
                <SnackbarProvider> {/* Display various snackbars */}
                  <BrowserRouter basename={baseRoute}>
                    <Routes>
                      <Route path={router.home} element={<HomeLayout/>}/>
                      <Route path={router.dashboards} element={<DashboardsLayout/>}/>
                    </Routes>
                  </BrowserRouter>
                </SnackbarProvider>
              </GraphStateProvider>
            </AuthStateProvider>
          </StateProvider>
        </ThemeProvider>
      </StyledEngineProvider>
    </LocalizationProvider>
  </>;
};

export default App;
