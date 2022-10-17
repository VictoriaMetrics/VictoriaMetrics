import {createTheme} from "@mui/material/styles";
import {getAppModeParams} from "../utils/app-mode";

const {palette} = getAppModeParams();

const THEME = createTheme({
  palette: {
    primary: {
      main: palette?.primary || "#3F51B5",
      light: "#e3f2fd"
    },
    secondary: {
      main: palette?.secondary || "#F50057"
    },
    error: {
      main: palette?.error || "#FF4141"
    },
    warning: {
      main: palette?.warning || "#ff9800"
    },
    info: {
      main: palette?.info || "#03a9f4"
    },
    success: {
      main: palette?.success || "#4caf50"
    }
  },
  components: {
    MuiFormHelperText: {
      styleOverrides: {
        root: {
          position: "absolute",
          bottom: "-16px",
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
          lineHeight: "1",
          zIndex: 0
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
          fontSize: "14px",
          boxShadow: "rgba(0, 0, 0, 0.08) 0px 4px 12px"
        }
      }
    },
    MuiTableCell: {
      styleOverrides: {
        head: {
          fontWeight: 600
        }
      }
    },
    MuiTab: {
      styleOverrides: {
        root: {
          fontWeight: 600
        }
      }
    }
  },
  typography: {
    "fontSize": 10,
    fontFamily: "'Lato', sans-serif"
  }
});

export default THEME;
