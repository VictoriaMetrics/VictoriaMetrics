const router = {
  home: "/",
  dashboards: "/dashboards",
  cardinality: "/cardinality",
  topQueries: "/top-queries",
  trace: "/trace",
  metrics: "/metrics",
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
    title: "Custom panel",
    ...routerOptionsDefault
  },
  [router.dashboards]: {
    title: "Dashboards",
    ...routerOptionsDefault,
  },
  [router.cardinality]: {
    title: "Cardinality",
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
  [router.metrics]: {
    title: "Explore",
    header: {
      timeSelector: true,
    }
  },
  [router.icons]: {
    title: "Icons",
    header: {}
  }
};

export default router;
