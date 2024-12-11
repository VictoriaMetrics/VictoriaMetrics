import React, { createContext, FC, useContext, useEffect, useState } from "preact/compat";
import Alert from "../components/Main/Alert/Alert";
import useDeviceDetect from "../hooks/useDeviceDetect";
import classNames from "classnames";
import { CloseIcon } from "../components/Main/Icons";
import { ReactNode } from "react";

interface SnackbarItem {
  text: string | ReactNode,
  type: "success" | "error" | "info" | "warning"
  timeout?: number
}

export interface SnackModel extends SnackbarItem {
  open?: boolean;
  key?: number;
}

type SnackbarContextType = {
  showInfoMessage: (item: SnackbarItem) => void
};

export const SnackbarContext = createContext<SnackbarContextType>({
  showInfoMessage: () => {
    // default value here makes no sense
  }
});

export const useSnack = (): SnackbarContextType => useContext(SnackbarContext);

export const SnackbarProvider: FC = ({ children }) => {
  const { isMobile } = useDeviceDetect();

  const [snack, setSnack] = useState<SnackModel>({ text: "", type: "info" });
  const [open, setOpen] = useState(false);

  const [infoMessage, setInfoMessage] = useState<SnackbarItem | null>(null);

  useEffect(() => {
    if (!infoMessage) return;
    setSnack({
      ...infoMessage,
      key: Date.now()
    });
    setOpen(true);
    const timeout = setTimeout(handleClose, infoMessage.timeout || 4000);

    return () => clearTimeout(timeout);
  }, [infoMessage]);

  const handleClose = () => {
    setInfoMessage(null);
    setOpen(false);
  };

  return <SnackbarContext.Provider value={{ showInfoMessage: setInfoMessage }}>
    {open && <div
      className={classNames({
        "vm-snackbar": true,
        "vm-snackbar_mobile": isMobile,
      })}
    >
      <Alert variant={snack.type}>
        <div className="vm-snackbar-content">
          <span>{snack.text}</span>
          <div
            className="vm-snackbar-content__close"
            onClick={handleClose}
          >
            <CloseIcon/>
          </div>
        </div>
      </Alert>
    </div>}
    {children}
  </SnackbarContext.Provider>;
};


