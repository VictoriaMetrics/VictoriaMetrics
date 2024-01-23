import { AppType } from "../types/appType";

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
  queryAnalyzer: "/query-analyzer",
  icons: "/icons",
  anomaly: "/anomaly",
  query: "/query",
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

const { REACT_APP_TYPE } = process.env;
const isLogsApp = REACT_APP_TYPE === AppType.logs;

const routerOptionsDefault = {
  header: {
    tenant: true,
    stepControl: !isLogsApp,
    timeSelector: !isLogsApp,
    executionControls: !isLogsApp,
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
  [router.queryAnalyzer]: {
    title: "Query analyzer",
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
  },
  [router.anomaly]: {
    title: "Anomaly exploration",
    ...routerOptionsDefault
  },
  [router.query]: {
    title: "Query",
    ...routerOptionsDefault
  }
};

export default router;
