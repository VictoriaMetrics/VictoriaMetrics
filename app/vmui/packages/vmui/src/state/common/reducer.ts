import { getDefaultServer } from "../../utils/default-server-url";
import { getQueryStringValue } from "../../utils/query-string";
import { getFromStorage, saveToStorage } from "../../utils/storage";

export interface AppState {
  serverUrl: string;
  tenantId: number;
  darkTheme: boolean
}

export type Action =
  | { type: "SET_SERVER", payload: string }
  | { type: "SET_TENANT_ID", payload: number }
  | { type: "SET_DARK_THEME", payload: boolean }

export const initialState: AppState = {
  serverUrl: getDefaultServer(),
  tenantId: Number(getQueryStringValue("g0.tenantID", 0)),
  darkTheme: !!getFromStorage("DARK_THEME")
};

export function reducer(state: AppState, action: Action): AppState {
  switch (action.type) {
    case "SET_SERVER":
      return {
        ...state,
        serverUrl: action.payload
      };
    case "SET_TENANT_ID":
      return {
        ...state,
        tenantId: action.payload
      };
    case "SET_DARK_THEME":
      saveToStorage("DARK_THEME", action.payload);
      return {
        ...state,
        darkTheme: action.payload
      };
    default:
      throw new Error();
  }
}
