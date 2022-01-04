import {createTheme} from "@mui/material/styles";

const THEME = createTheme({
  palette: {
    primary: {
      main: "#3F51B5"
    },
    secondary: {
      main: "#F50057"
    },
    error: {
      main: "#FF4141"
    }
  },
  components: {
    MuiFormHelperText: {
      styleOverrides: {
        root: {
          position: "absolute",
          top: "36px",
          left: "2px",
          margin: 0,
        }
      }
    },
    MuiInputLabel: {
      styleOverrides: {
        root: {
          fontSize: "12px",
          letterSpacing: "normal",
          lineHeight: "1"
        }
      }
    },
    MuiInputBase: {
      styleOverrides: {
        "root": {
          "&.Mui-focused fieldset": {
            "borderWidth": "1px !important"
          }
        }
      }
    },
    MuiSwitch: {
      defaultProps: {
        color: "secondary"
      }
    },
    MuiAccordion: {
      styleOverrides: {
        root: {
          boxShadow: "rgba(0, 0, 0, 0.16) 0px 1px 4px"
        },
      },
    },
    MuiPaper: {
      styleOverrides: {
        root: {
          boxShadow: "rgba(0, 0, 0, 0.2) 0px 3px 8px"
        },
      },
    },
    MuiButton: {
      styleOverrides: {
        contained: {
          boxShadow: "rgba(17, 17, 26, 0.1) 0px 0px 16px",
          "&:hover": {
            boxShadow: "rgba(0, 0, 0, 0.1) 0px 4px 12px",
          },
        }
      }
    },
    MuiIconButton: {
      defaultProps: {
        size: "large",
      },
      styleOverrides: {
        sizeLarge: {
          borderRadius: "20%",
          height: "40px",
          width: "41px"
        },
        sizeMedium: {
          borderRadius: "20%",
        },
        sizeSmall: {
          borderRadius: "20%",
        }
      }
    },
    MuiTooltip: {
      styleOverrides: {
        tooltip: {
          fontSize: "10px"
        }
      }
    },
    MuiAlert: {
      styleOverrides: {
        root: {
          boxShadow: "rgba(0, 0, 0, 0.08) 0px 4px 12px"
        }
      }
    }
  },
  typography: {
    "fontSize": 10
  }
});

export default THEME;