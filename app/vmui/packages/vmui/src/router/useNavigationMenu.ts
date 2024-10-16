import { getAppModeEnable } from "../utils/app-mode";
import { useDashboardsState } from "../state/dashboards/DashboardsStateContext";
import { useAppState } from "../state/common/StateContext";
import { useMemo } from "preact/compat";
import { AppType } from "../types/appType";
import { processNavigationItems } from "./utils";
import { getAnomalyNavigation, getDefaultNavigation, getLogsNavigation } from "./navigation";

const appType = process.env.REACT_APP_TYPE;

const useNavigationMenu = () => {
  const appModeEnable = getAppModeEnable();
  const { dashboardsSettings } = useDashboardsState();
  const { serverUrl, flags, appConfig } = useAppState();
  const isEnterpriseLicense = appConfig.license?.type === "enterprise";
  const showAlertLink = Boolean(flags["vmalert.proxyURL"]);
  const showPredefinedDashboards = Boolean(!appModeEnable && dashboardsSettings.length);

  const navigationConfig = useMemo(() => ({
    serverUrl,
    isEnterpriseLicense,
    showAlertLink,
    showPredefinedDashboards
  }), [serverUrl, isEnterpriseLicense, showAlertLink, showPredefinedDashboards]);


  const menu = useMemo(() => {
    switch (appType) {
      case AppType.logs:
        return getLogsNavigation();
      case AppType.anomaly:
        return getAnomalyNavigation();
      default:
        return getDefaultNavigation(navigationConfig);
    }
  }, [navigationConfig]);

  return processNavigationItems(menu);
};

export default useNavigationMenu;


