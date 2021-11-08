import React, {FC} from "react";
import {SnackbarProvider} from "./contexts/Snackbar";
import HomeLayout from "./components/Home/HomeLayout";
import {StateProvider} from "./state/common/StateContext";
import {AuthStateProvider} from "./state/auth/AuthStateContext";
import {GraphStateProvider} from "./state/graph/GraphStateContext";
import {MuiThemeProvider} from "@material-ui/core";
import {createTheme} from "@material-ui/core/styles";

import CssBaseline from "@material-ui/core/CssBaseline";

import {MuiPickersUtilsProvider} from "@material-ui/pickers";
// pick a date util library
import DayJsUtils from "@date-io/dayjs";

const App: FC = () => {

  const THEME = createTheme({
    typography: {
      "fontSize": 10
    }
  });

  return (
    <>
      <CssBaseline /> {/* CSS Baseline: kind of normalize.css made by materialUI team - can be scoped */}
      <MuiPickersUtilsProvider utils={DayJsUtils}> {/* Allows datepicker to work with DayJS */}
        <MuiThemeProvider theme={THEME}>  {/* Material UI theme customization */}
          <StateProvider> {/* Serialized into query string, common app settings */}
            <AuthStateProvider> {/* Auth related info - optionally persisted to Local Storage */}
              <GraphStateProvider> {/* Graph settings */}
                <SnackbarProvider> {/* Display various snackbars */}
                  <HomeLayout/>
                </SnackbarProvider>
              </GraphStateProvider>
            </AuthStateProvider>
          </StateProvider>
        </MuiThemeProvider>
      </MuiPickersUtilsProvider>
    </>
  );
};

export default App;
