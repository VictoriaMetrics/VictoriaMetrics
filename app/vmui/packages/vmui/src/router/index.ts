const router = {
  home: "/",
  metrics: "/metrics",
  dashboards: "/dashboards",
  cardinality: "/cardinality",
  topQueries: "/top-queries",
  trace: "/trace",
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

const routerOptionsDefault = {
  header: {
    tenant: true,
    stepControl: true,
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
  [router.icons]: {
    title: "Icons",
    header: {}
  }
};

export default router;
