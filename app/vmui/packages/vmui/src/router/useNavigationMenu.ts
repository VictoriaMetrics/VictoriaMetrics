import { getAppModeEnable } from "../utils/app-mode";
import { useDashboardsState } from "../state/dashboards/DashboardsStateContext";
import { useAppState } from "../state/common/StateContext";
import { useMemo } from "preact/compat";
import { processNavigationItems } from "./utils";
import { getAnomalyNavigation, getDefaultNavigation, getLogsNavigation, getAlertNavigation } from "./navigation";
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
        return getLogsNavigation(navigationConfig);
      case AppType.vmanomaly:
        return getAnomalyNavigation();
      case AppType.vmalert:
        return getAlertNavigation();
      default:
        return getDefaultNavigation(navigationConfig);
    }
  }, [navigationConfig]);

  return processNavigationItems(menu);
};

export default useNavigationMenu;


