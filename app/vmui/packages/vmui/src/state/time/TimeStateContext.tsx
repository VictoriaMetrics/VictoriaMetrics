import { createContext, FC, ReactNode, useContext, useMemo, useReducer, Dispatch } from "react";
import { TimeAction, TimeState, initialTimeState, reducer } from "./reducer";

type TimeStateContextType = { state: TimeState, dispatch: Dispatch<TimeAction> };

export const TimeStateContext = createContext<TimeStateContextType>({} as TimeStateContextType);

export const useTimeState = (): TimeState => useContext(TimeStateContext).state;
export const useTimeDispatch = (): Dispatch<TimeAction> => useContext(TimeStateContext).dispatch;

type Props = {
  children: ReactNode;
}

export const TimeStateProvider: FC<Props> = ({ children }) => {
  const [state, dispatch] = useReducer(reducer, initialTimeState);

  const contextValue = useMemo(() => {
    return { state, dispatch };
  }, [state, dispatch]);

  return <TimeStateContext.Provider value={contextValue}>
    {children}
  </TimeStateContext.Provider>;
};


