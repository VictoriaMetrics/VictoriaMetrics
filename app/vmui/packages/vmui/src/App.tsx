import React, {FC} from "preact/compat";
import {SnackbarProvider} from "./contexts/Snackbar";
import HomeLayout from "./components/Home/HomeLayout";
import {StateProvider} from "./state/common/StateContext";
import {AuthStateProvider} from "./state/auth/AuthStateContext";
import {GraphStateProvider} from "./state/graph/GraphStateContext";
import THEME from "./theme/theme";
import { ThemeProvider, StyledEngineProvider } from "@mui/material/styles";
import CssBaseline from "@mui/material/CssBaseline";
import LocalizationProvider from "@mui/lab/LocalizationProvider";
import DayjsUtils from "@date-io/dayjs";


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
                  <HomeLayout/>
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
