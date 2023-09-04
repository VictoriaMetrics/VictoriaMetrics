import React, { createContext, FC, useContext, useMemo, useReducer } from "preact/compat";
import { CustomPanelAction, CustomPanelState, initialCustomPanelState, reducer } from "./reducer";

import { Dispatch } from "react";

type CustomPanelStateContextType = { state: CustomPanelState, dispatch: Dispatch<CustomPanelAction> };

export const CustomPanelStateContext = createContext<CustomPanelStateContextType>({} as CustomPanelStateContextType);

export const useCustomPanelState = (): CustomPanelState => useContext(CustomPanelStateContext).state;
export const useCustomPanelDispatch = (): Dispatch<CustomPanelAction> => useContext(CustomPanelStateContext).dispatch;

export const CustomPanelStateProvider: FC = ({ children }) => {
  const [state, dispatch] = useReducer(reducer, initialCustomPanelState);

  const contextValue = useMemo(() => {
    return { state, dispatch };
  }, [state, dispatch]);

  return <CustomPanelStateContext.Provider value={contextValue}>
    {children}
  </CustomPanelStateContext.Provider>;
};


