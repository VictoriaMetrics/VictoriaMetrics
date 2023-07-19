const router = {
  home: "/",
  metrics: "/metrics",
  dashboards: "/dashboards",
  cardinality: "/cardinality",
  topQueries: "/top-queries",
  trace: "/trace",
  withTemplate: "/expand-with-exprs",
  relabel: "/relabeling",
  logs: "/logs",
  activeQueries: "/active-queries",
  icons: "/icons"
};

export interface RouterOptionsHeader {
  tenant?: boolean,
  stepControl?: boolean,
  timeSelector?: boolean,
  executionControls?: boolean,
  globalSettings?: boolean,
  cardinalityDatePicker?: boolean
}

export interface RouterOptions {
  title?: string,
  header: RouterOptionsHeader
}

const { REACT_APP_LOGS } = process.env;

const routerOptionsDefault = {
  header: {
    tenant: true,
    stepControl: !REACT_APP_LOGS,
    timeSelector: !REACT_APP_LOGS,
    executionControls: !REACT_APP_LOGS,
  }
};

export const routerOptions: {[key: string]: RouterOptions} = {
  [router.home]: {
    title: "Query",
    ...routerOptionsDefault
  },
  [router.metrics]: {
    title: "Explore Prometheus metrics",
    header: {
      tenant: true,
      stepControl: true,
      timeSelector: true,
    }
  },
  [router.cardinality]: {
    title: "Explore cardinality",
    header: {
      tenant: true,
      cardinalityDatePicker: true,
    }
  },
  [router.topQueries]: {
    title: "Top queries",
    header: {
      tenant: true,
    }
  },
  [router.trace]: {
    title: "Trace analyzer",
    header: {}
  },
  [router.dashboards]: {
    title: "Dashboards",
    ...routerOptionsDefault,
  },
  [router.withTemplate]: {
    title: "WITH templates",
    header: {}
  },
  [router.relabel]: {
    title: "Metric relabel debug",
    header: {}
  },
  [router.logs]: {
    title: "Logs Explorer",
    header: {}
  },
  [router.activeQueries]: {
    title: "Active Queries",
    header: {}
  },
  [router.icons]: {
    title: "Icons",
    header: {}
  }
};

export default router;
