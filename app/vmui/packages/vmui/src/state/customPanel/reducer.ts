import { DisplayType, displayTypeTabs } from "../../pages/CustomPanel/DisplayTypeSwitch";
import { getFromStorage, saveToStorage } from "../../utils/storage";
import { getQueryStringValue } from "../../utils/query-string";
import { SeriesLimits } from "../../types";
import { DEFAULT_MAX_SERIES } from "../../constants/graph";

export interface CustomPanelState {
  displayType: DisplayType;
  nocache: boolean;
  isTracingEnabled: boolean;
  seriesLimits: SeriesLimits
}

export type CustomPanelAction =
  | { type: "SET_DISPLAY_TYPE", payload: DisplayType }
  | { type: "SET_SERIES_LIMITS", payload: SeriesLimits }
  | { type: "TOGGLE_NO_CACHE"}
  | { type: "TOGGLE_QUERY_TRACING" }

const queryTab = getQueryStringValue("g0.tab", 0) as string;
const displayType = displayTypeTabs.find(t => t.prometheusCode === +queryTab || t.value === queryTab);
const limitsStorage = getFromStorage("SERIES_LIMITS") as string;

export const initialCustomPanelState: CustomPanelState = {
  displayType: (displayType?.value || "chart") as DisplayType,
  nocache: getFromStorage("NO_CACHE") as boolean || false,
  isTracingEnabled: getFromStorage("QUERY_TRACING") as boolean || false,
  seriesLimits: limitsStorage ? JSON.parse(getFromStorage("SERIES_LIMITS") as string) : DEFAULT_MAX_SERIES
};

export function reducer(state: CustomPanelState, action: CustomPanelAction): CustomPanelState {
  switch (action.type) {
    case "SET_DISPLAY_TYPE":
      return {
        ...state,
        displayType: action.payload
      };
    case "SET_SERIES_LIMITS":
      saveToStorage("SERIES_LIMITS", JSON.stringify(action.payload));
      return {
        ...state,
        seriesLimits: action.payload
      };
    case "TOGGLE_QUERY_TRACING":
      saveToStorage("QUERY_TRACING", !state.isTracingEnabled);
      return {
        ...state,
        isTracingEnabled: !state.isTracingEnabled,

      };
    case "TOGGLE_NO_CACHE":
      saveToStorage("NO_CACHE", !state.nocache);
      return {
        ...state,
        nocache: !state.nocache
      };
    default:
      throw new Error();
  }
}
