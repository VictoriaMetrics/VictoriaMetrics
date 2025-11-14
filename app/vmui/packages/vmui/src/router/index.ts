import { APP_TYPE, AppType } from "../constants/appType";

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
  anomaly: "/anomaly",
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
  executionControls?: ExecutionControlsProps;
  globalSettings?: boolean;
  cardinalityDatePicker?: boolean;
}

export interface RouterOptions {
  title?: string;
  header: RouterOptionsHeader;
}

interface ExecutionControlsProps {
  tooltip: string;
  useAutorefresh: boolean;
}

const routerOptionsDefault = {
  header: {
    tenant: true,
    stepControl: true,
    timeSelector: true,
    executionControls: {
      tooltip: "Refresh dashboard",
      useAutorefresh: true,
    }
  },
};

const getDefaultOptions = (appType: AppType) => {
  switch (appType) {
    case AppType.vmanomaly:
      return {
        title: "Anomaly exploration",
        ...routerOptionsDefault,
      };
    default:
      return {
        title: "Query",
        ...routerOptionsDefault,
      };
  }
};

export const routerOptions: { [key: string]: RouterOptions } = {
  [router.home]: getDefaultOptions(APP_TYPE),
  [router.rawQuery]: {
    title: "Raw query",
    header: {
      tenant: true,
      stepControl: false,
      timeSelector: true,
      executionControls: {
        tooltip: "Refresh dashboard",
        useAutorefresh: true,
      }
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
    header: {
      executionControls: {
        tooltip: "Refresh alerts",
        useAutorefresh: false,
      }
    },
  },
  [router.notifiers]: {
    title: "Notifiers",
    header: {
      executionControls: {
        tooltip: "Refresh notifiers",
        useAutorefresh: false,
      },
    },
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
  [router.anomaly]: getDefaultOptions(AppType.vmanomaly),
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
