/* eslint max-lines: ["error", {"max": 300}] */

import React, {useState} from "react";
import DialogTitle from "@material-ui/core/DialogTitle";
import Dialog from "@material-ui/core/Dialog";
import {
  Box,
  Button,
  Checkbox,
  createStyles,
  DialogActions,
  DialogContent,
  DialogContentText,
  FormControl,
  FormControlLabel,
  FormHelperText,
  Input,
  InputAdornment,
  InputLabel,
  Tab,
  Tabs,
  TextField,
  Typography
} from "@material-ui/core";
import TabPanel from "./AuthTabPanel";
import PersonIcon from "@material-ui/icons/Person";
import LockIcon from "@material-ui/icons/Lock";
import {makeStyles} from "@material-ui/core/styles";
import {useAuthDispatch, useAuthState} from "../../../state/auth/AuthStateContext";
import {AUTH_METHOD, WithCheckbox} from "../../../state/auth/reducer";

// TODO: make generic when creating second dialog
export interface DialogProps {
  open: boolean;
  onClose: () => void;
}

export interface AuthTab {
  title: string;
  id: AUTH_METHOD;
}

const useStyles = makeStyles(() =>
  createStyles({
    tabsContent: {
      height: "200px"
    },
  }),
);

const BEARER_PREFIX = "Bearer ";

const tabs: AuthTab[] = [
  {title: "No auth", id: "NO_AUTH"},
  {title: "Basic Auth", id: "BASIC_AUTH"},
  {title: "Bearer Token", id: "BEARER_AUTH"}
];

export const AuthDialog: React.FC<DialogProps> = (props) => {

  const classes = useStyles();
  const {onClose, open} = props;

  const {saveAuthLocally, basicData, bearerData, authMethod} = useAuthState();
  const dispatch = useAuthDispatch();

  const [authCheckbox, setAuthCheckbox] = useState(saveAuthLocally);

  const [basicValue, setBasicValue] = useState(basicData || {password: "", login: ""});

  const [bearerValue, setBearerValue] = useState(bearerData?.token || BEARER_PREFIX);

  const [tabIndex, setTabIndex] = useState(tabs.findIndex(el => el.id === authMethod) || 0);

  const handleChange = (event: unknown, newValue: number) => {
    setTabIndex(newValue);
  };

  const handleBearerChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const newVal = event.target.value;
    if (newVal.startsWith(BEARER_PREFIX)) {
      setBearerValue(newVal);
    } else {
      setBearerValue(BEARER_PREFIX);
    }
  };

  const handleClose = () => {
    onClose();
  };

  const onBearerPaste = (e: React.ClipboardEvent) => {
    // if you're pasting token word Bearer will be added automagically
    const newVal = e.clipboardData.getData("text/plain");
    if (newVal.startsWith(BEARER_PREFIX)) {
      setBearerValue(newVal);
    } else {
      setBearerValue(BEARER_PREFIX + newVal);
    }
    e.preventDefault();
  };

  const handleApply = () => {
    // TODO: handle validation/required fields
    switch (tabIndex) {
      case 0:
        dispatch({type: "SET_NO_AUTH", payload: {checkbox: authCheckbox} as WithCheckbox});
        break;
      case 1:
        dispatch({type: "SET_BASIC_AUTH", payload: { checkbox: authCheckbox, value: basicValue}});
        break;
      case 2:
        dispatch({type: "SET_BEARER_AUTH", payload: {checkbox: authCheckbox, value: {token: bearerValue}}});
        break;
    }
    handleClose();
  };

  return (
    <Dialog onClose={handleClose} aria-labelledby="simple-dialog-title" open={open}>
      <DialogTitle id="simple-dialog-title">Request Auth Settings</DialogTitle>
      <DialogContent>
        <DialogContentText>
          This affects Authorization header sent to the server you specify. Not shown in URL and can be optionally stored on a client side
        </DialogContentText>

        <Tabs
          value={tabIndex}
          onChange={handleChange}
          indicatorColor="primary"
          textColor="primary"
        >
          {
            tabs.map(t => <Tab key={t.id} label={t.title} />)
          }
        </Tabs>
        <Box p={0} display="flex" flexDirection="column" className={classes.tabsContent}>
          <Box flexGrow={1}>
            <TabPanel value={tabIndex} index={0}>
              <Typography style={{fontStyle: "italic"}}>
                No Authorization Header
              </Typography>
            </TabPanel>
            <TabPanel value={tabIndex} index={1}>
              <FormControl margin="dense" fullWidth={true}>
                <InputLabel htmlFor="basic-login">User</InputLabel>
                <Input
                  id="basic-login"
                  startAdornment={
                    <InputAdornment position="start">
                      <PersonIcon />
                    </InputAdornment>
                  }
                  required
                  onChange={e => setBasicValue(prev => ({...prev, login: e.target.value || ""}))}
                  value={basicValue?.login || ""}
                />
              </FormControl>
              <FormControl margin="dense" fullWidth={true}>
                <InputLabel htmlFor="basic-pass">Password</InputLabel>
                <Input
                  id="basic-pass"
                  // type="password" // Basic auth is not super secure in any case :)
                  startAdornment={
                    <InputAdornment position="start">
                      <LockIcon />
                    </InputAdornment>
                  }
                  onChange={e => setBasicValue(prev => ({...prev, password: e.target.value || ""}))}
                  value={basicValue?.password || ""}
                />
              </FormControl>
            </TabPanel>
            <TabPanel value={tabIndex} index={2}>
              <TextField
                id="bearer-auth"
                label="Bearer token"
                multiline
                fullWidth={true}
                value={bearerValue}
                onChange={handleBearerChange}
                InputProps={{
                  onPaste: onBearerPaste
                }}
                rowsMax={6}
              />
            </TabPanel>
          </Box>

          <FormControl>
            <FormControlLabel
              control={
                <Checkbox
                  checked={authCheckbox}
                  onChange={() => setAuthCheckbox(prev => !prev)}
                  name="checkedB"
                  color="primary"
                />
              }
              label="Persist Auth Data Locally"
            />
            <FormHelperText>
              {authCheckbox ? "Auth Data and the Selected method will be saved to LocalStorage" : "Auth Data won't be saved. All previously saved Auth Data will be removed"}
            </FormHelperText>
          </FormControl>

        </Box>

      </DialogContent>
      <DialogActions>
        <Button onClick={handleApply} color="primary">
          Apply
        </Button>
      </DialogActions>
    </Dialog>
  );
};