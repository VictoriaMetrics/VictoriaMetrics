import { createContext, FC, ReactNode, useContext, useMemo, useReducer, Dispatch } from "react";
import { CustomPanelAction, CustomPanelState, initialCustomPanelState, reducer } from "./reducer";

type CustomPanelStateContextType = { state: CustomPanelState, dispatch: Dispatch<CustomPanelAction> };

export const CustomPanelStateContext = createContext<CustomPanelStateContextType>({} as CustomPanelStateContextType);

export const useCustomPanelState = (): CustomPanelState => useContext(CustomPanelStateContext).state;
export const useCustomPanelDispatch = (): Dispatch<CustomPanelAction> => useContext(CustomPanelStateContext).dispatch;

type Props = {
  children: ReactNode;
}

export const CustomPanelStateProvider: FC<Props> = ({ children }) => {
  const [state, dispatch] = useReducer(reducer, initialCustomPanelState);

  const contextValue = useMemo(() => {
    return { state, dispatch };
  }, [state, dispatch]);

  return <CustomPanelStateContext.Provider value={contextValue}>
    {children}
  </CustomPanelStateContext.Provider>;
};


