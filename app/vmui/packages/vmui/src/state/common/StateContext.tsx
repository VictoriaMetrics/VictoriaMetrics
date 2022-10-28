import React, { createContext, FC, useContext, useMemo, useReducer } from "preact/compat";
import { Action, AppState, initialState, reducer } from "./reducer";
import { getQueryStringValue } from "../../utils/query-string";
import { Dispatch } from "react";

type StateContextType = { state: AppState, dispatch: Dispatch<Action> };

export const StateContext = createContext<StateContextType>({} as StateContextType);

export const useAppState = (): AppState => useContext(StateContext).state;
export const useAppDispatch = (): Dispatch<Action> => useContext(StateContext).dispatch;

export const initialPrepopulatedState = Object.entries(initialState)
  .reduce((acc, [key, value]) => ({
    ...acc,
    [key]: getQueryStringValue(key) || value
  }), {}) as AppState;

export const AppStateProvider: FC = ({ children }) => {
  const [state, dispatch] = useReducer(reducer, initialPrepopulatedState);

  const contextValue = useMemo(() => {
    return { state, dispatch };
  }, [state, dispatch]);

  return <StateContext.Provider value={contextValue}>
    {children}
  </StateContext.Provider>;
};


