import React, { createContext, FC, useContext, useEffect, useState } from "preact/compat";
import Alert from "../components/Main/Alert/Alert";

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
    // TODO: default value here makes no sense
  }
});

export const useSnack = (): SnackbarContextType => useContext(SnackbarContext);

export const SnackbarProvider: FC = ({ children }) => {
  const [snack, setSnack] = useState<SnackModel>({});
  const [open, setOpen] = useState(false);

  const [infoMessage, setInfoMessage] = useState<SnackbarItem>(undefined);

  useEffect(() => {
    if (!infoMessage) return;
    console.log(infoMessage);
    setSnack({
      message: infoMessage.text,
      variant: infoMessage.type,
      key: new Date().getTime()
    });
    setOpen(true);
    const timeout = setTimeout(handleClose, 4000);

    return () => clearTimeout(timeout);
  }, [infoMessage]);

  const handleClose = (e: unknown, reason: string): void => {
    if (reason !== "clickaway") {
      setInfoMessage(undefined);
      setOpen(false);
    }
  };

  return <SnackbarContext.Provider value={{ showInfoMessage: setInfoMessage }}>
    {open && <div className="vm-snackbar">
      <Alert variant={snack.variant}>
        {snack.message}
      </Alert>
    </div>}
    {children}
  </SnackbarContext.Provider>;
};


