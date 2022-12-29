import { DashboardSettings } from "../../types";

export interface DashboardsState {
  dashboardsSettings: DashboardSettings[];
  dashboardsLoading: boolean,
  dashboardsError: string
}

export type DashboardsAction =
  | { type: "SET_DASHBOARDS_SETTINGS", payload: DashboardSettings[] }
  | { type: "SET_DASHBOARDS_LOADING", payload: boolean }
  | { type: "SET_DASHBOARDS_ERROR", payload: string }


export const initialDashboardsState: DashboardsState = {
  dashboardsSettings: [],
  dashboardsLoading: false,
  dashboardsError: "",
};

export function reducer(state: DashboardsState, action: DashboardsAction): DashboardsState {
  switch (action.type) {
    case "SET_DASHBOARDS_SETTINGS":
      return {
        ...state,
        dashboardsSettings: action.payload
      };
    case "SET_DASHBOARDS_LOADING":
      return {
        ...state,
        dashboardsLoading: action.payload
      };
    case "SET_DASHBOARDS_ERROR":
      return {
        ...state,
        dashboardsError: action.payload
      };
    default:
      throw new Error();
  }
}
