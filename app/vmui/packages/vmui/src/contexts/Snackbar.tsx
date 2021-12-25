import React, {createContext, FC, useContext, useEffect, useState} from "preact/compat";
import Alert from "@mui/material/Alert";
import Snackbar from "@mui/material/Snackbar";

export interface SnackModel {
  message?: string;
  color?: string;
  open?: boolean;
  key?: number;
}

type SnackbarContextType = { showInfoMessage: (value: string) => void };

export const SnackbarContext = createContext<SnackbarContextType>({
  showInfoMessage: () => {
    // TODO: default value here makes no sense
  }
});

export const useSnack = (): SnackbarContextType => useContext(SnackbarContext);

export const SnackbarProvider: FC = ({children}) => {
  const [snack, setSnack] = useState<SnackModel>({});
  const [open, setOpen] = useState(false);

  const [infoMessage, setInfoMessage] = useState<string | undefined>(undefined);

  useEffect(() => {
    if (infoMessage) {
      setSnack({
        message: infoMessage,
        key: new Date().getTime()
      });
      setOpen(true);
    }
  }, [infoMessage]);

  const handleClose = (e: unknown, reason: string): void => {
    if (reason !== "clickaway") {
      setInfoMessage(undefined);
      setOpen(false);
    }
  };

  return <SnackbarContext.Provider value={{showInfoMessage: setInfoMessage}}>
    <Snackbar open={open} key={snack.key} autoHideDuration={4000} onClose={handleClose}>
      <Alert>
        {snack.message}
      </Alert>
    </Snackbar>
    {children}
  </SnackbarContext.Provider>;
};


