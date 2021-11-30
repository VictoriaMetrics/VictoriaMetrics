import React, {FC} from "react";
import {SnackbarProvider} from "./contexts/Snackbar";
import HomeLayout from "./components/Home/HomeLayout";
import {StateProvider} from "./state/common/StateContext";
import {AuthStateProvider} from "./state/auth/AuthStateContext";
import {GraphStateProvider} from "./state/graph/GraphStateContext";
import { ThemeProvider, Theme, StyledEngineProvider, createTheme } from "@mui/material/styles";

import CssBaseline from "@mui/material/CssBaseline";

import LocalizationProvider from "@mui/lab/LocalizationProvider";
// pick a date util library
import DayjsUtils from "@date-io/dayjs";


declare module "@mui/styles/defaultTheme" {
  // eslint-disable-next-line @typescript-eslint/no-empty-interface
  interface DefaultTheme extends Theme {}
}


const App: FC = () => {

  const THEME = createTheme({
    palette: {
      primary: {
        main: "#3F51B5"
      },
      secondary: {
        main: "#F50057"
      }
    },
    components: {
      MuiSwitch: {
        defaultProps: {
          color: "secondary"
        }
      },
      MuiAccordion: {
        styleOverrides: {
          root: {
            boxShadow: "rgba(0, 0, 0, 0.16) 0px 1px 4px;"
          },
        },
      }
    },
    typography: {
      "fontSize": 10
    }
  });

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
