const router = {
  home: "/",
  dashboards: "/dashboards",
  cardinality: "/cardinality",
  topQueries: "/top-queries",
  trace: "/trace"
};

export interface RouterOptions {
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
  [router.home]: routerOptionsDefault,
  [router.dashboards]: routerOptionsDefault,
  [router.cardinality]: {
    header: {
      cardinalityDatePicker: true,
    }
  }
};

export default router;
