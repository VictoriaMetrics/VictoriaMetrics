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
 * Special case for alert link
 */
const getAlertLink = (url: string, showAlertLink: boolean) => {
  // see more https://docs.victoriametrics.com/cluster-victoriametrics/?highlight=vmalertproxyurl#vmalert
  return {
    label: "Alerts",
    value: `${url}/vmalert`,
    type: NavigationItemType.externalLink,
    hide: !showAlertLink,
  };
};

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
 * Default navigation menu
 */
export const getDefaultNavigation = ({
  serverUrl,
  isEnterpriseLicense,
  showPredefinedDashboards,
  showAlertLink,
}: NavigationConfig): NavigationItem[] => [
  { value: router.home },
  { label: "Explore", submenu: getExploreNav() },
  { label: "Tools", submenu: getToolsNav(isEnterpriseLicense) },
  { value: router.dashboards, hide: !showPredefinedDashboards },
  getAlertLink(serverUrl, showAlertLink),
];

/**
 * VictoriaLogs navigation menu
 */
export const getLogsNavigation = (): NavigationItem[] => [
  {
    label: routerOptions[router.logs].title,
    value: router.home,
  },
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
