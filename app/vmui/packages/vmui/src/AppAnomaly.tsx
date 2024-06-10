import React, { FC, useState } from "preact/compat";
import { HashRouter, Route, Routes } from "react-router-dom";
import AppContextProvider from "./contexts/AppContextProvider";
import ThemeProvider from "./components/Main/ThemeProvider/ThemeProvider";
import AnomalyLayout from "./layouts/AnomalyLayout/AnomalyLayout";
import ExploreAnomaly from "./pages/ExploreAnomaly/ExploreAnomaly";
import router from "./router";
import CustomPanel from "./pages/CustomPanel";

const AppLogs: FC = () => {
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
                element={<AnomalyLayout/>}
              >
                <Route
                  path={"/"}
                  element={<ExploreAnomaly/>}
                />
                <Route
                  path={router.query}
                  element={<CustomPanel/>}
                />
              </Route>
            </Routes>
          )}
        </>
      </AppContextProvider>
    </HashRouter>
  </>;
};

export default AppLogs;
