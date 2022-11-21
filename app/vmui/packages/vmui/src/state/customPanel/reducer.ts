import { DisplayType, displayTypeTabs } from "../../pages/CustomPanel/DisplayTypeSwitch";
import { getFromStorage, saveToStorage } from "../../utils/storage";
import { getQueryStringValue } from "../../utils/query-string";

export interface CustomPanelState {
  displayType: DisplayType;
  nocache: boolean;
  isTracingEnabled: boolean;
  tableCompact: boolean;
}

export type CustomPanelAction =
  | { type: "SET_DISPLAY_TYPE", payload: DisplayType }
  | { type: "TOGGLE_NO_CACHE"}
  | { type: "TOGGLE_QUERY_TRACING" }
  | { type: "TOGGLE_TABLE_COMPACT" }

const queryTab = getQueryStringValue("g0.tab", 0) as string;
const displayType = displayTypeTabs.find(t => t.prometheusCode === +queryTab || t.value === queryTab);

export const initialCustomPanelState: CustomPanelState = {
  displayType: (displayType?.value || "chart") as DisplayType,
  nocache: getFromStorage("NO_CACHE") as boolean || false,
  isTracingEnabled: getFromStorage("QUERY_TRACING") as boolean || false,
  tableCompact: getFromStorage("TABLE_COMPACT") as boolean || false,
};

export function reducer(state: CustomPanelState, action: CustomPanelAction): CustomPanelState {
  switch (action.type) {
    case "SET_DISPLAY_TYPE":
      return {
        ...state,
        displayType: action.payload
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
    case "TOGGLE_TABLE_COMPACT":
      saveToStorage("TABLE_COMPACT", !state.tableCompact);
      return {
        ...state,
        tableCompact: !state.tableCompact
      };
    default:
      throw new Error();
  }
}
