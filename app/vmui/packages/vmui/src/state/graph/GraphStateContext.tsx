import React, { createContext, FC, useContext, useMemo, useReducer } from "preact/compat";
import { GraphAction, GraphState, initialGraphState, reducer } from "./reducer";
import { Dispatch } from "react";

type GraphStateContextType = { state: GraphState, dispatch: Dispatch<GraphAction> };

export const GraphStateContext = createContext<GraphStateContextType>({} as GraphStateContextType);

export const useGraphState = (): GraphState => useContext(GraphStateContext).state;
export const useGraphDispatch = (): Dispatch<GraphAction> => useContext(GraphStateContext).dispatch;

export const GraphStateProvider: FC = ({ children }) => {
  const [state, dispatch] = useReducer(reducer, initialGraphState);

  const contextValue = useMemo(() => {
    return { state, dispatch };
  }, [state, dispatch]);

  return <GraphStateContext.Provider value={contextValue}>
    {children}
  </GraphStateContext.Provider>;
};


