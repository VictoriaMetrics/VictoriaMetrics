import React, { FC, useState } from "preact/compat";
import { HashRouter, Route, Routes } from "react-router-dom";
import router from "./router";
import AppContextProvider from "./contexts/AppContextProvider";
import Layout from "./components/Layout/Layout";
import CustomPanel from "./pages/CustomPanel";
import DashboardsLayout from "./pages/PredefinedPanels";
import CardinalityPanel from "./pages/CardinalityPanel";
import TopQueries from "./pages/TopQueries";
import ThemeProvider from "./components/Main/ThemeProvider/ThemeProvider";
import Spinner from "./components/Main/Spinner/Spinner";
import TracePage from "./pages/TracePage";
import ExploreMetrics from "./pages/ExploreMetrics";
import PreviewIcons from "./components/Main/Icons/PreviewIcons";

const App: FC = () => {

  const [loadingTheme, setLoadingTheme] = useState(true);

  return <>
    {loadingTheme && <Spinner/>}
    <ThemeProvider setLoadingTheme={setLoadingTheme}/>

    <HashRouter key={`${loadingTheme}`}>
      <AppContextProvider>
        <Routes>
          <Route
            path={"/"}
            element={<Layout/>}
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
              path={router.dashboards}
              element={<DashboardsLayout/>}
            />
            <Route
              path={router.icons}
              element={<PreviewIcons/>}
            />
          </Route>
        </Routes>
      </AppContextProvider>
    </HashRouter>
  </>;
};

export default App;
