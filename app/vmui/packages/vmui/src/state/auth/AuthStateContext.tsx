import React, {createContext, FC, useContext, useMemo, useReducer} from "preact/compat";
import {AuthAction, AuthState, initialPrepopulatedState, reducer} from "./reducer";
import {Dispatch} from "react";

type AuthStateContextType = { state: AuthState, dispatch: Dispatch<AuthAction> };

export const AuthStateContext = createContext<AuthStateContextType>({} as AuthStateContextType);

export const useAuthState = (): AuthState => useContext(AuthStateContext).state;
export const useAuthDispatch = (): Dispatch<AuthAction> => useContext(AuthStateContext).dispatch;

export const AuthStateProvider: FC = ({children}) => {

  const [state, dispatch] = useReducer(reducer, initialPrepopulatedState);

  const contextValue = useMemo(() => {
    return { state, dispatch };
  }, [state, dispatch]);


  return <AuthStateContext.Provider value={contextValue}>
    {children}
  </AuthStateContext.Provider>;
};


