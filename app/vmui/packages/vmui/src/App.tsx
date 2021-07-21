import React, {FC} from "react";
import {SnackbarProvider} from "./contexts/Snackbar";
import HomeLayout from "./components/Home/HomeLayout";
import {StateProvider} from "./state/common/StateContext";
import {AuthStateProvider} from "./state/auth/AuthStateContext";
import {createMuiTheme, MuiThemeProvider} from "@material-ui/core";

import CssBaseline from "@material-ui/core/CssBaseline";

import {MuiPickersUtilsProvider} from "@material-ui/pickers";
// pick a date util library
import DayJsUtils from "@date-io/dayjs";

const App: FC = () => {

  const THEME = createMuiTheme({
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
              <SnackbarProvider> {/* Display various snackbars */}
                <HomeLayout/>
              </SnackbarProvider>
            </AuthStateProvider>
          </StateProvider>
        </MuiThemeProvider>
      </MuiPickersUtilsProvider>
    </>
  );
};

export default App;
