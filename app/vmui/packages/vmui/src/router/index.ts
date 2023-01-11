const router = {
  home: "/",
  metrics: "/metrics",
  dashboards: "/dashboards",
  cardinality: "/cardinality",
  topQueries: "/top-queries",
  trace: "/trace",
  icons: "/icons"
};

export interface RouterOptions {
  title?: string,
  header: {
    timeSelector?: boolean,
    executionControls?: boolean,
    globalSettings?: boolean,
    cardinalityDatePicker?: boolean
  }
}

const routerOptionsDefault = {
  header: {
    timeSelector: true,
    executionControls: true,
  }
};

export const routerOptions: {[key: string]: RouterOptions} = {
  [router.home]: {
    title: "Query",
    ...routerOptionsDefault
  },
  [router.metrics]: {
    title: "Explore metrics",
    header: {
      timeSelector: true,
    }
  },
  [router.cardinality]: {
    title: "Explore cardinality",
    header: {
      cardinalityDatePicker: true,
    }
  },
  [router.topQueries]: {
    title: "Top queries",
    header: {}
  },
  [router.trace]: {
    title: "Trace analyzer",
    header: {}
  },
  [router.dashboards]: {
    title: "Dashboards",
    ...routerOptionsDefault,
  },
  [router.icons]: {
    title: "Icons",
    header: {}
  }
};

export default router;
