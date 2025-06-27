import React, { FC, useState } from "preact/compat";
import { HashRouter, Route, Routes } from "react-router-dom";
import AppContextProvider from "./contexts/AppContextProvider";
import ThemeProvider from "./components/Main/ThemeProvider/ThemeProvider";
import ExploreRules from "./pages/ExploreAlerts/ExploreRules";
import ExploreNotifiers from "./pages/ExploreAlerts/ExploreNotifiers";
import AlertLayout from "./layouts/AlertLayout/AlertLayout";
import "./constants/markedPlugins";

const AppAlert: FC = () => {
  const [loadedTheme, setLoadedTheme] = useState(false);

  return <>
    <HashRouter>
      <AppContextProvider>
        <>
          <ThemeProvider onLoaded={setLoadedTheme}/>
          {loadedTheme && (
            <Routes>
              <Route
                path={"/"}
                element={<AlertLayout/>}
              >
                <Route
                  path={"/"}
                  element={<ExploreRules/>}
                />
                <Route
                  path={"/rules"}
                  element={<ExploreRules/>}
                />
                <Route
                  path={"/notifiers"}
                  element={<ExploreNotifiers/>}
                />
              </Route>
            </Routes>
          )}
        </>
      </AppContextProvider>
    </HashRouter>
  </>;
};

export default AppAlert;
