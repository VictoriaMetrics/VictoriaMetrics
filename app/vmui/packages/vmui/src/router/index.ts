
const router = {
  home: "/",
  metrics: "/metrics",
  dashboards: "/dashboards",
  cardinality: "/cardinality",
  topQueries: "/top-queries",
  trace: "/trace",
  withTemplate: "/expand-with-exprs",
  relabel: "/relabeling",
  activeQueries: "/active-queries",
  queryAnalyzer: "/query-analyzer",
  icons: "/icons",
  query: "/query",
  rawQuery: "/raw-query",
  downsamplingDebug: "/downsampling-filters-debug",
  retentionDebug: "/retention-filters-debug",
  rules: "/rules",
  notifiers: "/notifiers",
};

export interface RouterOptionsHeader {
  tenant?: boolean;
  stepControl?: boolean;
  timeSelector?: boolean;
  globalSettings?: boolean;
  cardinalityDatePicker?: boolean;
}

export interface RouterOptions {
  title?: string;
  header: RouterOptionsHeader;
}

const routerOptionsDefault = {
  header: {
    tenant: true,
    stepControl: true,
    timeSelector: true,
  },
};

export const routerOptions: { [key: string]: RouterOptions } = {
  [router.home]: {
    title: "Query",
    ...routerOptionsDefault,
  },
  [router.rawQuery]: {
    title: "Raw query",
    header: {
      tenant: true,
      stepControl: false,
      timeSelector: true,
    },
  },
  [router.metrics]: {
    title: "Explore Prometheus metrics",
    header: {
      tenant: true,
      stepControl: true,
      timeSelector: true,
    },
  },
  [router.cardinality]: {
    title: "Explore cardinality",
    header: {
      tenant: true,
      cardinalityDatePicker: true,
    },
  },
  [router.topQueries]: {
    title: "Top queries",
    header: {
      tenant: true,
    },
  },
  [router.trace]: {
    title: "Trace analyzer",
    header: {},
  },
  [router.queryAnalyzer]: {
    title: "Query analyzer",
    header: {},
  },
  [router.dashboards]: {
    title: "Dashboards",
    ...routerOptionsDefault,
  },
  [router.rules]: {
    title: "Rules",
    header: {},
  },
  [router.notifiers]: {
    title: "Notifiers",
    header: {},
  },
  [router.withTemplate]: {
    title: "WITH templates",
    header: {},
  },
  [router.relabel]: {
    title: "Metric relabel debug",
    header: {},
  },
  [router.activeQueries]: {
    title: "Active Queries",
    header: {},
  },
  [router.icons]: {
    title: "Icons",
    header: {},
  },
  [router.query]: {
    title: "Query",
    ...routerOptionsDefault,
  },
  [router.downsamplingDebug]: {
    title: "Downsampling filters debug",
    header: {},
  },
  [router.retentionDebug]: {
    title: "Retention filters debug",
    header: {},
  },
};

export default router;
