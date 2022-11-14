import React, { FC, useState } from "preact/compat";
import { HashRouter, Route, Routes } from "react-router-dom";
import router from "./router";
import AppContextProvider from "./contexts/AppContextProvider";
import HomeLayout from "./components/Home/HomeLayout";
import CustomPanel from "./pages/CustomPanel";
import DashboardsLayout from "./pages/PredefinedPanels";
import CardinalityPanel from "./pages/CardinalityPanel";
import TopQueries from "./pages/TopQueries";
import ThemeProvider from "./components/Main/ThemeProvider/ThemeProvider";
import Spinner from "./components/Main/Spinner/Spinner";

const App: FC = () => {

  const [loadingTheme, setLoadingTheme] = useState(true);

  if (loadingTheme) return (
    <>
      <Spinner/>
      <ThemeProvider setLoadingTheme={setLoadingTheme}/>;
    </>
  );

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
