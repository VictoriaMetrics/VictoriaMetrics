import React, { createContext, FC, useContext, useMemo, useReducer } from "preact/compat";
import { Action, TopQueriesState, initialState, reducer } from "./reducer";
import { Dispatch } from "react";
type TopQueriesStateContextType = { state: TopQueriesState, dispatch: Dispatch<Action> };

export const TopQueriesStateContext = createContext<TopQueriesStateContextType>({} as TopQueriesStateContextType);

export const useTopQueriesState = (): TopQueriesState => useContext(TopQueriesStateContext).state;
export const useTopQueriesDispatch = (): Dispatch<Action> => useContext(TopQueriesStateContext).dispatch;

export const TopQueriesStateProvider: FC = ({ children }) => {
  const [state, dispatch] = useReducer(reducer, initialState);

  const contextValue = useMemo(() => {
    return { state, dispatch };
  }, [state, dispatch]);

  return <TopQueriesStateContext.Provider value={contextValue}>
    {children}
  </TopQueriesStateContext.Provider>;
};


