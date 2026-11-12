import { createContext, FC, useContext, useEffect, useMemo, useReducer } from "preact/compat";
import { Action, AppState, initialState, reducer } from "./reducer";
import { Dispatch } from "react";
import { getFromStorage, removeFromStorage, saveToStorage } from "../../utils/storage";

type StateContextType = { state: AppState, dispatch: Dispatch<Action> };

export const StateContext = createContext<StateContextType>({} as StateContextType);

export const useAppState = (): AppState => useContext(StateContext).state;
export const useAppDispatch = (): Dispatch<Action> => useContext(StateContext).dispatch;

export const AppStateProvider: FC = ({ children }) => {
  const [state, dispatch] = useReducer(reducer, initialState);

  const contextValue = useMemo(() => {
    return { state, dispatch };
  }, [state, dispatch]);

  useEffect(() => {
    if (!state.serverUrl) return;
    const enabledStorage = !!getFromStorage("SERVER_URL");

    if (enabledStorage) {
      saveToStorage("SERVER_URL", state.serverUrl);
    } else {
      removeFromStorage(["SERVER_URL"]);
    }
  }, [state.serverUrl]);

  return <StateContext.Provider value={contextValue}>
    {children}
  </StateContext.Provider>;
};


