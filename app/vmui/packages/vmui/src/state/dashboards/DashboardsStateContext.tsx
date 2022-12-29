import React, { createContext, FC, useContext, useMemo, useReducer } from "preact/compat";
import { DashboardsAction, DashboardsState, initialDashboardsState, reducer } from "./reducer";

import { Dispatch } from "react";

type DashboardsStateContextType = { state: DashboardsState, dispatch: Dispatch<DashboardsAction> };

export const DashboardsStateContext = createContext<DashboardsStateContextType>({} as DashboardsStateContextType);

export const useDashboardsState = (): DashboardsState => useContext(DashboardsStateContext).state;
export const useDashboardsDispatch = (): Dispatch<DashboardsAction> => useContext(DashboardsStateContext).dispatch;
export const DashboardsStateProvider: FC = ({ children }) => {
  const [state, dispatch] = useReducer(reducer, initialDashboardsState);

  const contextValue = useMemo(() => {
    return { state, dispatch };
  }, [state, dispatch]);

  return <DashboardsStateContext.Provider value={contextValue}>
    {children}
  </DashboardsStateContext.Provider>;
};


