import React, { createContext, FC, useContext, useMemo, useReducer } from "preact/compat";
import { Action, CardinalityState, initialState, reducer } from "./reducer";
import { Dispatch } from "react";

type CardinalityStateContextType = { state: CardinalityState, dispatch: Dispatch<Action> };

export const CardinalityStateContext = createContext<CardinalityStateContextType>({} as CardinalityStateContextType);

export const useCardinalityState = (): CardinalityState => useContext(CardinalityStateContext).state;
export const useCardinalityDispatch = (): Dispatch<Action> => useContext(CardinalityStateContext).dispatch;

export const CardinalityStateProvider: FC = ({ children }) => {
  const [state, dispatch] = useReducer(reducer, initialState);

  const contextValue = useMemo(() => {
    return { state, dispatch };
  }, [state, dispatch]);

  return <CardinalityStateContext.Provider value={contextValue}>
    {children}
  </CardinalityStateContext.Provider>;
};


