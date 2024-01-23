import React, { FC, useState } from "preact/compat";
import { HashRouter, Route, Routes } from "react-router-dom";
import router from "./router";
import AppContextProvider from "./contexts/AppContextProvider";
import MainLayout from "./layouts/MainLayout/MainLayout";
import CustomPanel from "./pages/CustomPanel";
import DashboardsLayout from "./pages/PredefinedPanels";
import CardinalityPanel from "./pages/CardinalityPanel";
import TopQueries from "./pages/TopQueries";
import ThemeProvider from "./components/Main/ThemeProvider/ThemeProvider";
import TracePage from "./pages/TracePage";
import ExploreMetrics from "./pages/ExploreMetrics";
import PreviewIcons from "./components/Main/Icons/PreviewIcons";
import WithTemplate from "./pages/WithTemplate";
import Relabel from "./pages/Relabel";
import ActiveQueries from "./pages/ActiveQueries";
import QueryAnalyzer from "./pages/QueryAnalyzer";

const App: FC = () => {
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
                element={<MainLayout/>}
              >
                <Route
                  path={router.home}
                  element={<CustomPanel/>}
                />
                <Route
                  path={router.metrics}
                  element={<ExploreMetrics/>}
                />
                <Route
                  path={router.cardinality}
                  element={<CardinalityPanel/>}
                />
                <Route
                  path={router.topQueries}
                  element={<TopQueries/>}
                />
                <Route
                  path={router.trace}
                  element={<TracePage/>}
                />
                <Route
                  path={router.queryAnalyzer}
                  element={<QueryAnalyzer/>}
                />
                <Route
                  path={router.dashboards}
                  element={<DashboardsLayout/>}
                />
                <Route
                  path={router.withTemplate}
                  element={<WithTemplate/>}
                />
                <Route
                  path={router.relabel}
                  element={<Relabel/>}
                />
                <Route
                  path={router.activeQueries}
                  element={<ActiveQueries/>}
                />
                <Route
                  path={router.icons}
                  element={<PreviewIcons/>}
                />
              </Route>
            </Routes>
          )}
        </>
      </AppContextProvider>
    </HashRouter>
  </>;
};

export default App;
