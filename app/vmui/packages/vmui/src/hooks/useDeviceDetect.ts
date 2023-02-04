import { useEffect, useState } from "react";
import { isMobileAgent } from "../utils/detect-device";
import useResize from "./useResize";

export default function useDeviceDetect() {
  const windowSize = useResize(document.body);
  const [isMobile, setMobile] = useState(false);

  useEffect(() => {
    const mobileAgent = isMobileAgent();
    const smallWidth = window.innerWidth < 500;
    setMobile(mobileAgent || smallWidth);
  }, [windowSize]);

  return { isMobile };
}
