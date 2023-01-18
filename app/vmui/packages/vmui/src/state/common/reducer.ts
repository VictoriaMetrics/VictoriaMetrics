import { getDefaultServer } from "../../utils/default-server-url";
import { getQueryStringValue } from "../../utils/query-string";

export interface AppState {
  serverUrl: string;
  tenantId: string;
}

export type Action =
  | { type: "SET_SERVER", payload: string }
  | { type: "SET_TENANT_ID", payload: string }

const tenantId = getQueryStringValue("g0.tenantID", "") as string;

export const initialState: AppState = {
  serverUrl: getDefaultServer(tenantId),
  tenantId,
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
    default:
      throw new Error();
  }
}
