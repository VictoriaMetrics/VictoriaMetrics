import { getAppModeEnable } from "../utils/app-mode";
import { useDashboardsState } from "../state/dashboards/DashboardsStateContext";
import { useAppState } from "../state/common/StateContext";
import { useMemo } from "preact/compat";
import { processNavigationItems } from "./utils";
import { getAnomalyNavigation, getDefaultNavigation, getLogsNavigation } from "./navigation";
import { APP_TYPE, AppType } from "../constants/appType";

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
    switch (APP_TYPE) {
      case AppType.victorialogs:
        return getLogsNavigation();
      case AppType.vmanomaly:
        return getAnomalyNavigation();
      default:
        return getDefaultNavigation(navigationConfig);
    }
  }, [navigationConfig]);

  return processNavigationItems(menu);
};

export default useNavigationMenu;


