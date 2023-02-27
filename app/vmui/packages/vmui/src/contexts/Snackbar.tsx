import React, { createContext, FC, useContext, useEffect, useState } from "preact/compat";
import Alert from "../components/Main/Alert/Alert";
import useDeviceDetect from "../hooks/useDeviceDetect";
import classNames from "classnames";
import { CloseIcon } from "../components/Main/Icons";

export interface SnackModel {
  message?: string;
  open?: boolean;
  key?: number;
  variant?: "success" | "error" | "info" | "warning";
}

type SnackbarItem = undefined | {
  text: string,
  type: "success" | "error" | "info" | "warning"
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

  const [snack, setSnack] = useState<SnackModel>({});
  const [open, setOpen] = useState(false);

  const [infoMessage, setInfoMessage] = useState<SnackbarItem>(undefined);

  useEffect(() => {
    if (!infoMessage) return;
    setSnack({
      message: infoMessage.text,
      variant: infoMessage.type,
      key: Date.now()
    });
    setOpen(true);
    const timeout = setTimeout(handleClose, 4000);

    return () => clearTimeout(timeout);
  }, [infoMessage]);

  const handleClose = () => {
    setInfoMessage(undefined);
    setOpen(false);
  };

  return <SnackbarContext.Provider value={{ showInfoMessage: setInfoMessage }}>
    {open && <div
      className={classNames({
        "vm-snackbar": true,
        "vm-snackbar_mobile": isMobile,
      })}
    >
      <Alert variant={snack.variant}>
        <div className="vm-snackbar-content">
          <span>{snack.message}</span>
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


