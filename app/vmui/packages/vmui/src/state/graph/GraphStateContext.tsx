import { createContext, FC, ReactNode, useContext, useMemo, useReducer, Dispatch } from "react";
import { GraphAction, GraphState, initialGraphState, reducer } from "./reducer";

type GraphStateContextType = { state: GraphState, dispatch: Dispatch<GraphAction> };

export const GraphStateContext = createContext<GraphStateContextType>({} as GraphStateContextType);

export const useGraphState = (): GraphState => useContext(GraphStateContext).state;
export const useGraphDispatch = (): Dispatch<GraphAction> => useContext(GraphStateContext).dispatch;

type Props = {
  children: ReactNode;
}

export const GraphStateProvider: FC<Props> = ({ children }) => {
  const [state, dispatch] = useReducer(reducer, initialGraphState);

  const contextValue = useMemo(() => {
    return { state, dispatch };
  }, [state, dispatch]);

  return <GraphStateContext.Provider value={contextValue}>
    {children}
  </GraphStateContext.Provider>;
};


