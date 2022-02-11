import React, {FC} from "preact/compat";
import Fade from "@mui/material/Fade";
import Box from "@mui/material/Box";
import CircularProgress from "@mui/material/CircularProgress";

interface SpinnerProps {
  isLoading: boolean;
  height?: string;
}

const Spinner: FC<SpinnerProps> = ({isLoading, height}) => {
  return <Fade in={isLoading} style={{
    transitionDelay: isLoading ? "300ms" : "0ms",
  }}>
    <Box alignItems="center" justifyContent="center" flexDirection="column" display="flex"
      style={{
        width: "100%",
        maxWidth: "calc(100vw - 64px)",
        position: "absolute",
        height: height ?? "50%",
        background: "rgba(255, 255, 255, 0.7)",
        pointerEvents: "none",
        zIndex: 2,
      }}>
      <CircularProgress/>
    </Box>
  </Fade>;
};

export default Spinner;