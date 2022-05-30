const router = {
  home: "/",
  dashboards: "/dashboards",
  cardinality: "/cardinality",
};

export interface RouterOptions {
  header: {
    timeSelector?: boolean,
    executionControls?: boolean,
    globalSettings?: boolean,
    datePicker?: boolean
  }
}

const routerOptionsDefault = {
  header: {
    timeSelector: true,
    executionControls: true,
    globalSettings: true,
  }
};

export const routerOptions: {[key: string]: RouterOptions} = {
  [router.home]: routerOptionsDefault,
  [router.dashboards]: routerOptionsDefault,
  [router.cardinality]: {
    header: {
      datePicker: true,
      globalSettings: true,
    }
  }
};

export default router;
