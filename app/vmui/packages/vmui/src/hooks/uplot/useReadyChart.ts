import { useState } from "preact/compat";
import uPlot from "uplot";
import useDragChart from "./useDragChart";
import { SetMinMax } from "../../types";

const isLiftClickWithMeta = (e: MouseEvent) => {
  const { ctrlKey, metaKey, button } = e;
  const leftClick = button === 0;
  return leftClick && (ctrlKey || metaKey);
};

// coefficient for drag speed; the higher the value, the faster the graph moves.
const dragSpeed = 0.9;

const useReadyChart = (setPlotScale: SetMinMax) => {
  const [isPanning, setPanning] = useState(false);

  const dragChart = useDragChart({ dragSpeed, setPanning, setPlotScale });

  const onReadyChart = (u: uPlot): void => {
    const handleInteractionStart = (e: MouseEvent | TouchEvent) => {
      const dragByMouse = e instanceof MouseEvent && isLiftClickWithMeta(e);
      const dragByTouch = window.TouchEvent && e instanceof TouchEvent && e.touches.length > 1;
      if (dragByMouse || dragByTouch) {
        dragChart({ u, e });
      }
    };

    const handleWheel = (e: WheelEvent) => {
      if (!e.ctrlKey && !e.metaKey) return;
      e.preventDefault();
      const { width } = u.over.getBoundingClientRect();
      const zoomPos = u.cursor.left && u.cursor.left > 0 ? u.cursor.left : 0;
      const xVal = u.posToVal(zoomPos, "x");
      const oxRange = (u.scales.x.max || 0) - (u.scales.x.min || 0);
      const nxRange = e.deltaY < 0 ? oxRange * dragSpeed : oxRange / dragSpeed;
      const min = xVal - (zoomPos / width) * nxRange;
      const max = min + nxRange;
      u.batch(() => setPlotScale({ min, max }));
    };

    u.over.addEventListener("mousedown", handleInteractionStart);
    u.over.addEventListener("touchstart", handleInteractionStart);
    u.over.addEventListener("wheel", handleWheel);
  };

  return {
    onReadyChart,
    isPanning,
  };
};

export default useReadyChart;
