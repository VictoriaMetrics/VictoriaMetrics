import {createTheme} from "@mui/material/styles";

const THEME = createTheme({
  palette: {
    primary: {
      main: "#3F51B5"
    },
    secondary: {
      main: "#F50057"
    }
  },
  components: {
    MuiSwitch: {
      defaultProps: {
        color: "secondary"
      }
    },
    MuiAccordion: {
      styleOverrides: {
        root: {
          boxShadow: "rgba(0, 0, 0, 0.16) 0px 1px 4px;"
        },
      },
    },
    MuiPaper: {
      styleOverrides: {
        elevation3: {
          boxShadow: "rgba(0, 0, 0, 0.2) 0px 3px 8px;"
        },
      },
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
    }
  },
  typography: {
    "fontSize": 10
  }
});

export default THEME;