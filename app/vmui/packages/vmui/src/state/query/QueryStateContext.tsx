import { createContext, FC, ReactNode, useContext, useMemo, useReducer, Dispatch } from "react";
import { QueryAction, QueryState, initialQueryState, reducer } from "./reducer";

type QueryStateContextType = { state: QueryState, dispatch: Dispatch<QueryAction> };

export const QueryStateContext = createContext<QueryStateContextType>({} as QueryStateContextType);

export const useQueryState = (): QueryState => useContext(QueryStateContext).state;
export const useQueryDispatch = (): Dispatch<QueryAction> => useContext(QueryStateContext).dispatch;

type Props = {
  children: ReactNode;
}

export const QueryStateProvider: FC<Props> = ({ children }) => {
  const [state, dispatch] = useReducer(reducer, initialQueryState);

  const contextValue = useMemo(() => {
    return { state, dispatch };
  }, [state, dispatch]);

  return <QueryStateContext.Provider value={contextValue}>
    {children}
  </QueryStateContext.Provider>;
};


