import { createContext, FC, ReactNode, useContext, useMemo, useReducer, Dispatch } from "react";
import { DashboardsAction, DashboardsState, initialDashboardsState, reducer } from "./reducer";

type DashboardsStateContextType = { state: DashboardsState, dispatch: Dispatch<DashboardsAction> };

export const DashboardsStateContext = createContext<DashboardsStateContextType>({} as DashboardsStateContextType);

export const useDashboardsState = (): DashboardsState => useContext(DashboardsStateContext).state;
export const useDashboardsDispatch = (): Dispatch<DashboardsAction> => useContext(DashboardsStateContext).dispatch;

type Props = {
  children: ReactNode;
}

export const DashboardsStateProvider: FC<Props> = ({ children }) => {
  const [state, dispatch] = useReducer(reducer, initialDashboardsState);

  const contextValue = useMemo(() => {
    return { state, dispatch };
  }, [state, dispatch]);

  return <DashboardsStateContext.Provider value={contextValue}>
    {children}
  </DashboardsStateContext.Provider>;
};


