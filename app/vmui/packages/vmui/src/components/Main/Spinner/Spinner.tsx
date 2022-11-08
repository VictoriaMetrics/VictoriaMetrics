import React, { FC } from "preact/compat";
import { ReactNode } from "react";

interface SpinnerProps {
  containerStyles?: Record<string, string | number>;
  title?: string | ReactNode,
}

export const defaultContainerStyles: Record<string, string | number> = {
  width: "100%",
  maxWidth: "calc(100vw - 64px)",
  height: "50%",
  position: "absolute",
  background: "rgba(255, 255, 255, 0.7)",
  pointerEvents: "none",
  zIndex: 2,
};

const Spinner: FC<SpinnerProps> = ({ containerStyles, title }) => {
  const styles = containerStyles ?? defaultContainerStyles;

  return <div style={styles}>
    {title}
  </div>;
  // return <Fade
  //   in={isLoading}
  //   style={{
  //     transitionDelay: isLoading ? "300ms" : "0ms",
  //   }}
  // >
  //   <Box
  //     alignItems="center"
  //     justifyContent="center"
  //     flexDirection="column"
  //     display="flex"
  //     style={styles}
  //   >
  //     <CircularProgress/>
  //     {title}
  //   </Box>
  // </Fade>;
};

export default Spinner;
