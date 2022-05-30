import React, {createContext, FC, useContext, useEffect, useMemo, useReducer} from "preact/compat";
import {Action, CardinalityState, initialState, reducer} from "./reducer";
import {Dispatch} from "react";
import {useLocation} from "react-router-dom";
import {setQueryStringValue} from "../../utils/query-string";
import router from "../../router";

type CardinalityStateContextType = { state: CardinalityState, dispatch: Dispatch<Action> };

export const CardinalityStateContext = createContext<CardinalityStateContextType>({} as CardinalityStateContextType);

export const useCardinalityState = (): CardinalityState => useContext(CardinalityStateContext).state;
export const useCardinalityDispatch = (): Dispatch<Action> => useContext(CardinalityStateContext).dispatch;

export const CardinalityStateProvider: FC = ({children}) => {
  const location = useLocation();

  const [state, dispatch] = useReducer(reducer, initialState);

  useEffect(() => {
    if (location.pathname !== router.cardinality) return;
    setQueryStringValue(state as unknown as Record<string, unknown>);
  }, [state, location]);

  const contextValue = useMemo(() => {
    return { state, dispatch };
  }, [state, dispatch]);


  return <CardinalityStateContext.Provider value={contextValue}>
    {children}
  </CardinalityStateContext.Provider>;
};


