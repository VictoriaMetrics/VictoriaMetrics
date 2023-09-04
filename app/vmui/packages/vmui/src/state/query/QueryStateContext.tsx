import React, { createContext, FC, useContext, useMemo, useReducer } from "preact/compat";
import { QueryAction, QueryState, initialQueryState, reducer } from "./reducer";

import { Dispatch } from "react";

type QueryStateContextType = { state: QueryState, dispatch: Dispatch<QueryAction> };

export const QueryStateContext = createContext<QueryStateContextType>({} as QueryStateContextType);

export const useQueryState = (): QueryState => useContext(QueryStateContext).state;
export const useQueryDispatch = (): Dispatch<QueryAction> => useContext(QueryStateContext).dispatch;

export const QueryStateProvider: FC = ({ children }) => {
  const [state, dispatch] = useReducer(reducer, initialQueryState);

  const contextValue = useMemo(() => {
    return { state, dispatch };
  }, [state, dispatch]);

  return <QueryStateContext.Provider value={contextValue}>
    {children}
  </QueryStateContext.Provider>;
};


