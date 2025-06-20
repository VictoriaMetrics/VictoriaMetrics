import { createContext, FC, ReactNode, useContext, useMemo, useReducer, Dispatch } from "react";
import { Action, AppState, initialState, reducer } from "./reducer";
import { getQueryStringValue } from "../../utils/query-string";

type StateContextType = { state: AppState, dispatch: Dispatch<Action> };

export const StateContext = createContext<StateContextType>({} as StateContextType);

export const useAppState = (): AppState => useContext(StateContext).state;
export const useAppDispatch = (): Dispatch<Action> => useContext(StateContext).dispatch;

export const initialPrepopulatedState = Object.entries(initialState)
  .reduce((acc, [key, value]) => ({
    ...acc,
    [key]: getQueryStringValue(key) || value
  }), {}) as AppState;

type Props = {
  children: ReactNode;
}

export const AppStateProvider: FC<Props> = ({ children }) => {
  const [state, dispatch] = useReducer(reducer, initialPrepopulatedState);

  const contextValue = useMemo(() => {
    return { state, dispatch };
  }, [state, dispatch]);

  return <StateContext.Provider value={contextValue}>
    {children}
  </StateContext.Provider>;
};


