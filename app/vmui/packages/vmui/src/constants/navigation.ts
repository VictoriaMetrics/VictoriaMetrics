import router, { routerOptions } from "../router";

export interface NavigationItem {
  label?: string,
  value?: string,
  hide?: boolean
  submenu?: NavigationItem[],
}

export const logsNavigation: NavigationItem[] = [
  {
    label: routerOptions[router.logs].title,
    value: router.home,
  },
];

export const defaultNavigation: NavigationItem[] = [
  {
    label: routerOptions[router.home].title,
    value: router.home,
  },
  {
    label: "Explore",
    submenu: [
      {
        label: routerOptions[router.metrics].title,
        value: router.metrics,
      },
      {
        label: routerOptions[router.cardinality].title,
        value: router.cardinality,
      },
      {
        label: routerOptions[router.topQueries].title,
        value: router.topQueries,
      },
      {
        label: routerOptions[router.activeQueries].title,
        value: router.activeQueries,
      },
    ]
  },
  {
    label: "Tools",
    submenu: [
      {
        label: routerOptions[router.trace].title,
        value: router.trace,
      },
      {
        label: routerOptions[router.withTemplate].title,
        value: router.withTemplate,
      },
      {
        label: routerOptions[router.relabel].title,
        value: router.relabel,
      },
    ]
  }
];
