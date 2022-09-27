import React, {createContext, FC, useContext, useEffect, useMemo, useReducer} from "preact/compat";
import {Action, TopQueriesState, initialState, reducer} from "./reducer";
import {Dispatch} from "react";
import {useLocation} from "react-router-dom";
import {setQueryStringValue} from "../../utils/query-string";
import router from "../../router";

type TopQueriesStateContextType = { state: TopQueriesState, dispatch: Dispatch<Action> };

export const TopQueriesStateContext = createContext<TopQueriesStateContextType>({} as TopQueriesStateContextType);

export const useTopQueriesState = (): TopQueriesState => useContext(TopQueriesStateContext).state;
export const useTopQueriesDispatch = (): Dispatch<Action> => useContext(TopQueriesStateContext).dispatch;

export const TopQueriesStateProvider: FC = ({children}) => {
  const location = useLocation();

  const [state, dispatch] = useReducer(reducer, initialState);

  useEffect(() => {
    if (location.pathname !== router.topQueries) return;
    setQueryStringValue(state as unknown as Record<string, unknown>);
  }, [state, location]);

  const contextValue = useMemo(() => {
    return { state, dispatch };
  }, [state, dispatch]);


  return <TopQueriesStateContext.Provider value={contextValue}>
    {children}
  </TopQueriesStateContext.Provider>;
};


