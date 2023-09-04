import React, { createContext, FC, useContext, useMemo, useReducer } from "preact/compat";
import { TimeAction, TimeState, initialTimeState, reducer } from "./reducer";

import { Dispatch } from "react";

type TimeStateContextType = { state: TimeState, dispatch: Dispatch<TimeAction> };

export const TimeStateContext = createContext<TimeStateContextType>({} as TimeStateContextType);

export const useTimeState = (): TimeState => useContext(TimeStateContext).state;
export const useTimeDispatch = (): Dispatch<TimeAction> => useContext(TimeStateContext).dispatch;

export const TimeStateProvider: FC = ({ children }) => {
  const [state, dispatch] = useReducer(reducer, initialTimeState);

  const contextValue = useMemo(() => {
    return { state, dispatch };
  }, [state, dispatch]);

  return <TimeStateContext.Provider value={contextValue}>
    {children}
  </TimeStateContext.Provider>;
};


