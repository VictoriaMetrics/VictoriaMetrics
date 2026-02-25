import { getDefaultServer } from "../../utils/default-server-url";
import { getFromStorage, saveToStorage } from "../../utils/storage";
import { AppConfig, Theme } from "../../types";
import { isDarkTheme } from "../../utils/theme";
import { removeTrailingSlash } from "../../utils/url";
import { getTenantIdFromUrl } from "../../utils/tenants";

export interface AppState {
  serverUrl: string;
  tenantId: string;
  theme: Theme;
  isDarkTheme: boolean | null;
  appConfig: AppConfig
}

export type Action =
  | { type: "SET_SERVER", payload: string }
  | { type: "SET_THEME", payload: Theme }
  | { type: "SET_APP_CONFIG", payload: AppConfig }
  | { type: "SET_DARK_THEME" }

const serverUrl = removeTrailingSlash(getDefaultServer());

export const initialState: AppState = {
  serverUrl,
  tenantId: getTenantIdFromUrl(serverUrl),
  theme: (getFromStorage("THEME") || Theme.system) as Theme,
  isDarkTheme: null,
  appConfig: {}
};

export function reducer(state: AppState, action: Action): AppState {
  switch (action.type) {
    case "SET_SERVER":
      return {
        ...state,
        tenantId: getTenantIdFromUrl(action.payload),
        serverUrl: removeTrailingSlash(action.payload)
      };
    case "SET_THEME":
      saveToStorage("THEME", action.payload);
      return {
        ...state,
        theme: action.payload,
      };
    case "SET_DARK_THEME":
      return {
        ...state,
        isDarkTheme: isDarkTheme(state.theme)
      };
    case "SET_APP_CONFIG":
      return {
        ...state,
        appConfig: action.payload
      };
    default:
      throw new Error();
  }
}
