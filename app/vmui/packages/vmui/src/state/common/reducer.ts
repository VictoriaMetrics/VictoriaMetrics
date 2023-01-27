import { getDefaultServer } from "../../utils/default-server-url";
import { getQueryStringValue } from "../../utils/query-string";
import { getFromStorage, saveToStorage } from "../../utils/storage";

export interface AppState {
  serverUrl: string;
  tenantId: string;
  darkTheme: boolean
}

export type Action =
  | { type: "SET_SERVER", payload: string }
  | { type: "SET_DARK_THEME", payload: boolean }
  | { type: "SET_TENANT_ID", payload: string }

const tenantId = getQueryStringValue("g0.tenantID", "") as string;

export const initialState: AppState = {
  serverUrl: getDefaultServer(tenantId),
  tenantId,
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
