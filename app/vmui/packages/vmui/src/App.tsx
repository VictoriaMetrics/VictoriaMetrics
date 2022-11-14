import React, { FC, useEffect } from "preact/compat";
import { HashRouter, Route, Routes } from "react-router-dom";
import router from "./router";
import AppContextProvider from "./contexts/AppContextProvider";
import HomeLayout from "./components/Home/HomeLayout";
import CustomPanel from "./pages/CustomPanel";
import DashboardsLayout from "./pages/PredefinedPanels";
import CardinalityPanel from "./pages/CardinalityPanel";
import TopQueries from "./pages/TopQueries";

const App: FC = () => {

  useEffect(() => {
    const { innerWidth, innerHeight } = window;
    const { clientWidth, clientHeight } = document.documentElement;
    document.documentElement.style.setProperty("--scrollbar-width", (innerWidth - clientWidth) + "px");
    document.documentElement.style.setProperty("--scrollbar-height", (innerHeight - clientHeight) + "px");
  }, []);

  return <>
    <HashRouter>
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
    </HashRouter>
  </>;
};

export default App;
