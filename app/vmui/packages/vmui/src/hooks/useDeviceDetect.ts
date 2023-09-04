import { useEffect, useState } from "react";
import { isMobileAgent } from "../utils/detect-device";
import useWindowSize from "./useWindowSize";

export default function useDeviceDetect() {
  const windowSize = useWindowSize();

  const getIsMobile = () => {
    const mobileAgent = isMobileAgent();
    const smallWidth = window.innerWidth < 500;
    return mobileAgent || smallWidth;
  };

  const [isMobile, setMobile] = useState(getIsMobile());

  useEffect(() => {
    setMobile(getIsMobile());
  }, [windowSize]);

  return { isMobile };
}
