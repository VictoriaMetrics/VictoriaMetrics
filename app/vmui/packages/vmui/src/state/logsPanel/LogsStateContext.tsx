import { createContext, FC, ReactNode, useContext, useMemo, useReducer, Dispatch } from "react";
import { LogsAction, LogsState, initialLogsState, reducer } from "./reducer";

type LogsStateContextType = { state: LogsState, dispatch: Dispatch<LogsAction> };

export const LogsStateContext = createContext<LogsStateContextType>({} as LogsStateContextType);

export const useLogsState = (): LogsState => useContext(LogsStateContext).state;
export const useLogsDispatch = (): Dispatch<LogsAction> => useContext(LogsStateContext).dispatch;

type Props = {
  children: ReactNode;
}

export const LogsStateProvider: FC<Props> = ({ children }) => {
  const [state, dispatch] = useReducer(reducer, initialLogsState);

  const contextValue = useMemo(() => {
    return { state, dispatch };
  }, [state, dispatch]);

  return <LogsStateContext.Provider value={contextValue}>
    {children}
  </LogsStateContext.Provider>;
};


