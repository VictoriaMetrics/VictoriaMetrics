import { displayTypeTabs } from "../../pages/CustomPanel/DisplayTypeSwitch";
import { getQueryStringValue } from "../../utils/query-string";
import { getFromStorage, saveToStorage } from "../../utils/storage";
import { DisplayType, SeriesLimits } from "../../types";
import { DEFAULT_MAX_SERIES } from "../../constants/graph";

export interface CustomPanelState {
  displayType: DisplayType;
  nocache: boolean;
  isTracingEnabled: boolean;
  seriesLimits: SeriesLimits
  tableCompact: boolean;
  reduceMemUsage: boolean;
}

export type CustomPanelAction =
  | { type: "SET_DISPLAY_TYPE", payload: DisplayType }
  | { type: "SET_SERIES_LIMITS", payload: SeriesLimits }
  | { type: "TOGGLE_NO_CACHE"}
  | { type: "TOGGLE_QUERY_TRACING" }
  | { type: "TOGGLE_TABLE_COMPACT" }
  | { type: "TOGGLE_REDUCE_MEM_USAGE"}

export const getInitialDisplayType = () => {
  const queryTab = getQueryStringValue("g0.tab", 0) as string;
  const displayType = displayTypeTabs.find(t => t.prometheusCode === +queryTab || t.value === queryTab);
  return displayType?.value || DisplayType.chart;
};

const limitsStorage = getFromStorage("SERIES_LIMITS") as string;

export const initialCustomPanelState: CustomPanelState = {
  displayType: getInitialDisplayType(),
  nocache: false,
  isTracingEnabled: false,
  seriesLimits: limitsStorage ? JSON.parse(limitsStorage) : DEFAULT_MAX_SERIES,
  tableCompact: getFromStorage("TABLE_COMPACT") as boolean || false,
  reduceMemUsage: false
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
      return {
        ...state,
        isTracingEnabled: !state.isTracingEnabled,

      };
    case "TOGGLE_NO_CACHE":
      return {
        ...state,
        nocache: !state.nocache
      };
    case "TOGGLE_TABLE_COMPACT":
      saveToStorage("TABLE_COMPACT", !state.tableCompact);
      return {
        ...state,
        tableCompact: !state.tableCompact
      };
    case "TOGGLE_REDUCE_MEM_USAGE":
      saveToStorage("TABLE_COMPACT", !state.reduceMemUsage);
      return {
        ...state,
        reduceMemUsage: !state.reduceMemUsage
      };
    default:
      throw new Error();
  }
}
