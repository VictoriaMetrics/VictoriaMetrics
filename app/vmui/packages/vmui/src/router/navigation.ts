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
  showAlerting: boolean,
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
 * Submenu for Alerting tab
 */

const getAlertingNav = () => [
  { value: router.rules },
  { value: router.notifiers },
];

/**
 * Default navigation menu
 */
export const getDefaultNavigation = ({
  isEnterpriseLicense,
  showPredefinedDashboards,
  showAlerting,
}: NavigationConfig): NavigationItem[] => [
  { value: router.home },
  { value: router.rawQuery },
  { label: "Explore", submenu: getExploreNav() },
  { label: "Tools", submenu: getToolsNav(isEnterpriseLicense) },
  { value: router.dashboards, hide: !showPredefinedDashboards },
  { value: "Alerting", submenu: getAlertingNav(), hide: !showAlerting },
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
