import { FC, useState } from "preact/compat";
import { HashRouter, Route, Routes } from "react-router-dom";
import AppContextProvider from "./contexts/AppContextProvider";
import ThemeProvider from "./components/Main/ThemeProvider/ThemeProvider";
import ExploreRules from "./pages/ExploreAlerts/ExploreRules";
import ExploreNotifiers from "./pages/ExploreAlerts/ExploreNotifiers";
import AlertLayout from "./layouts/AlertLayout/AlertLayout";
import router from "./router";

const AppAlert: FC = () => {
  const [loadedTheme, setLoadedTheme] = useState(false);
  const prefix = "/vmalert";

  if (!window.location.hash.startsWith("#/")) {
    let idx = window.location.pathname.lastIndexOf(prefix);
    if (idx >= 0) {
      idx += prefix.length;
      const path = window.location.pathname.substring(0, idx);
      const urlParams = new URLSearchParams(window.location.search);
      let page = window.location.pathname.substring(idx);
      let hash = window.location.hash;
      if (page == "/rule") {
        page = "/groups";
        if (urlParams.has("rule_id")) {
          hash = `#rule-${urlParams.get("rule_id")}`;
        }
      } else if (page == "/alert") {
        page = "/alerts";
        if (urlParams.has("alert_id")) {
          hash = `#alert-${urlParams.get("alert_id")}`;
        }
      }
      hash = page + hash;
      window.history.replaceState(null, "", `${path}#${hash}`);
    }
  }

  return <>
    <HashRouter>
      <AppContextProvider>
        <>
          <ThemeProvider onLoaded={setLoadedTheme}/>
          {loadedTheme && (
            <Routes>
              <Route
                path="/"
                element={<AlertLayout/>}
              >
                <Route
                  path="/"
                  element={<ExploreRules
                    ruleTypeFilter=""
                  />}
                />
                <Route
                  path={router.rules}
                  element={<ExploreRules
                    ruleTypeFilter=""
                  />}
                />
                <Route
                  path={router.alerts}
                  element={<ExploreRules
                    ruleTypeFilter="alert"
                  />}
                />
                <Route
                  path={router.notifiers}
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
