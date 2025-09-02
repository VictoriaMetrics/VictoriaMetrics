import { getAppModeEnable } from "../utils/app-mode";
import { useDashboardsState } from "../state/dashboards/DashboardsStateContext";
import { useAppState } from "../state/common/StateContext";
import { useMemo } from "preact/compat";
import { processNavigationItems } from "./utils";
import { getAnomalyNavigation, getDefaultNavigation } from "./navigation";
import { APP_TYPE, AppType } from "../constants/appType";

const useNavigationMenu = () => {
  const appModeEnable = getAppModeEnable();
  const { dashboardsSettings } = useDashboardsState();
  const { serverUrl, appConfig } = useAppState();
  const isEnterpriseLicense = appConfig.license?.type === "enterprise";
  const showAlerting = appConfig?.vmalert?.enabled || false;
  const showPredefinedDashboards = Boolean(!appModeEnable && dashboardsSettings.length);

  const navigationConfig = useMemo(() => ({
    serverUrl,
    isEnterpriseLicense,
    showAlerting,
    showPredefinedDashboards
  }), [serverUrl, isEnterpriseLicense, showAlerting, showPredefinedDashboards]);


  const menu = useMemo(() => {
    switch (APP_TYPE) {
      case AppType.vmanomaly:
        return getAnomalyNavigation();
      default:
        return getDefaultNavigation(navigationConfig);
    }
  }, [navigationConfig]);

  return processNavigationItems(menu);
};

export default useNavigationMenu;


