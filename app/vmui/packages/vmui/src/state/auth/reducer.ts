import {authKeys, getFromStorage, removeFromStorage, saveToStorage} from "../../utils/storage";

export type AUTH_METHOD = "NO_AUTH" | "BASIC_AUTH" | "BEARER_AUTH";

export type BasicAuthData = {
  login: string;
  password: string;
};

export type BearerAuthData = {
  token: string; // "Bearer xxx"
};

export interface AuthState {
  authMethod: AUTH_METHOD;
  basicData?: BasicAuthData;
  bearerData?: BearerAuthData;
  saveAuthLocally: boolean;
}

export type WithCheckbox<T = undefined> = {checkbox: boolean; value: T};

export type AuthAction =
    | { type: "SET_BASIC_AUTH", payload: WithCheckbox<BasicAuthData> }
    | { type: "SET_BEARER_AUTH", payload: WithCheckbox<BearerAuthData> }
    | { type: "SET_NO_AUTH", payload: WithCheckbox}

export const initialState: AuthState = {
  authMethod: "NO_AUTH",
  saveAuthLocally: false
};

const initialAuthMethodData = getFromStorage("AUTH_TYPE") as AUTH_METHOD;
const initialBasicAuthData = getFromStorage("BASIC_AUTH_DATA") as BasicAuthData;
const initialBearerAuthData = getFromStorage("BEARER_AUTH_DATA") as BearerAuthData;

export const initialPrepopulatedState: AuthState = {
  ...initialState,
  authMethod: initialAuthMethodData || initialState.authMethod,
  basicData: initialBasicAuthData,
  bearerData: initialBearerAuthData,
  saveAuthLocally: !!(initialBasicAuthData || initialBearerAuthData)
};

export const removeAuthKeys = (): void => {
  removeFromStorage(authKeys);
};

export function reducer(state: AuthState, action: AuthAction): AuthState {
  // Reducer should not have side effects
  // but until auth storage is handled ONLY HERE,
  // it should be fine
  switch (action.type) {
    case "SET_BASIC_AUTH":
      action.payload.checkbox ? saveToStorage("BASIC_AUTH_DATA", action.payload.value) : removeAuthKeys();
      saveToStorage("AUTH_TYPE", "BASIC_AUTH");
      return {
        ...state,
        authMethod: "BASIC_AUTH",
        basicData: action.payload.value
      };
    case "SET_BEARER_AUTH":
      action.payload.checkbox ? saveToStorage("BEARER_AUTH_DATA", action.payload.value) : removeAuthKeys();
      saveToStorage("AUTH_TYPE", "BEARER_AUTH");
      return {
        ...state,
        authMethod: "BEARER_AUTH",
        bearerData: action.payload.value
      };
    case "SET_NO_AUTH":
      !action.payload.checkbox && removeAuthKeys();
      saveToStorage("AUTH_TYPE", "NO_AUTH");
      return {
        ...state,
        authMethod: "NO_AUTH"
      };
    default:
      throw new Error();
  }
}
