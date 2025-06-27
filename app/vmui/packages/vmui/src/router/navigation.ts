import router, { routerOptions } from "./index";

export enum NavigationItemType {
  internalLink,
  externalLink,
}

export interface NavigationItem {
  label?: string,
  value?: string,
  hide?: boolean
  submenu?: NavigationItem[],
  type?: NavigationItemType,
}

interface NavigationConfig {
  serverUrl: string,
  isEnterpriseLicense: boolean,
  showPredefinedDashboards: boolean,
  showAlertLink: boolean,
}

/**
 * Submenu for Tools tab
 */
const getToolsNav = (isEnterpriseLicense: boolean) => [
  { value: router.trace },
  { value: router.queryAnalyzer },
  { value: router.withTemplate },
  { value: router.relabel },
  { value: router.downsamplingDebug, hide: !isEnterpriseLicense },
  { value: router.retentionDebug, hide: !isEnterpriseLicense },
];

/**
 * Submenu for Explore tab
 */
const getExploreNav = () => [
  { value: router.metrics },
  { value: router.cardinality },
  { value: router.topQueries },
  { value: router.activeQueries },
];

/**
 * Submenu for Alerts tab
 */

const getAlertsNav = () => [
  { value: router.alertRules },
  { value: router.alertNotifiers },
];

/**
 * Default navigation menu
 */
export const getDefaultNavigation = ({
  isEnterpriseLicense,
  showPredefinedDashboards,
  showAlertLink,
}: NavigationConfig): NavigationItem[] => [
  { value: router.home },
  { value: router.rawQuery },
  { label: "Explore", submenu: getExploreNav() },
  { label: "Tools", submenu: getToolsNav(isEnterpriseLicense) },
  { value: router.dashboards, hide: !showPredefinedDashboards },
  { value: "Alerts", submenu: getAlertsNav(), hide: !showAlertLink },
];

/**
 * VictoriaLogs navigation menu
 */
export const getLogsNavigation = ({
  showAlertLink,
}): NavigationItem[] => [
  {
    label: routerOptions[router.logs].title,
    value: router.home,
  },
  { value: "Alerts", submenu: getAlertsNav(), hide: !showAlertLink },
];

/**
 * vmanomaly navigation menu
 */
export const getAnomalyNavigation = (): NavigationItem[] => [
  {
    label: routerOptions[router.anomaly].title,
    value: router.home,
  },
];

/**
 * VMAlert navigation menu
 */
export const getAlertNavigation = (): NavigationItem[] => [
  {
    label: routerOptions[router.alertRules].title,
    value: router.home,
  },
  {
    label: routerOptions[router.alertNotifiers].title,
    value: router.alertNotifiers,
  }
];
