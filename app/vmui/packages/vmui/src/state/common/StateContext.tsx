import React, {createContext, FC, useContext, useEffect, useMemo, useReducer} from "preact/compat";
import {Action, AppState, initialState, reducer} from "./reducer";
import {getQueryStringValue, setQueryStringValue} from "../../utils/query-string";
import {Dispatch} from "react";

type StateContextType = { state: AppState, dispatch: Dispatch<Action> };

export const StateContext = createContext<StateContextType>({} as StateContextType);

export const useAppState = (): AppState => useContext(StateContext).state;
export const useAppDispatch = (): Dispatch<Action> => useContext(StateContext).dispatch;

export const initialPrepopulatedState = Object.entries(initialState)
  .reduce((acc, [key, value]) => ({
    ...acc,
    [key]: getQueryStringValue(key) || value
  }), {}) as AppState;

export const StateProvider: FC = ({children}) => {

  const [state, dispatch] = useReducer(reducer, initialPrepopulatedState);

  useEffect(() => {
    setQueryStringValue(state as unknown as Record<string, unknown>);
  }, [state]);

  const contextValue = useMemo(() => {
    return { state, dispatch };
  }, [state, dispatch]);


  return <StateContext.Provider value={contextValue}>
    {children}
  </StateContext.Provider>;
};


