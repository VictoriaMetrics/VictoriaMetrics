import { useCallback, useState } from "preact/compat";
import useEventListener from "../useEventListener";
import { MinMax, SetMinMax } from "../../types";

interface ZoomChartHook {
  uPlotInst?: uPlot;
  xRange: MinMax;
  setPlotScale: SetMinMax;
}

const calculateDistance = (touches: TouchList) => {
  const dx = touches[0].clientX - touches[1].clientX;
  const dy = touches[0].clientY - touches[1].clientY;
  return Math.sqrt(dx * dx + dy * dy);
};

const useZoomChart = ({ uPlotInst, xRange, setPlotScale }: ZoomChartHook) => {
  const [startTouchDistance, setStartTouchDistance] = useState(0);

  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    const { target, ctrlKey, metaKey, key } = e;
    const isInput = target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement;
    if (!uPlotInst || isInput) return;

    const isPlus = key === "+" || key === "=";
    const isMinus = key === "-";
    const isNotControlKey = !(ctrlKey || metaKey);

    if ((isMinus || isPlus) && isNotControlKey) {
      e.preventDefault();
      const factor = (xRange.max - xRange.min) / 10 * (isPlus ? 1 : -1);
      setPlotScale({ min: xRange.min + factor, max: xRange.max - factor });
    }
  }, [uPlotInst, xRange]);

  const handleTouchStart = (e: TouchEvent) => {
    if (e.touches.length === 2) {
      e.preventDefault();
      setStartTouchDistance(calculateDistance(e.touches));
    }
  };

  const handleTouchMove = useCallback((e: TouchEvent) => {
    if (!uPlotInst || e.touches.length !== 2) return;
    e.preventDefault();

    const endTouchDistance = calculateDistance(e.touches);
    const diffDistance = startTouchDistance - endTouchDistance;

    const max = (uPlotInst.scales.x.max || xRange.max);
    const min = (uPlotInst.scales.x.min || xRange.min);
    const dur = max - min;
    const dir = (diffDistance > 0 ? -1 : 1);

    const zoomFactor = dur / 50 * dir;
    uPlotInst.batch(() => setPlotScale({ min: min + zoomFactor, max: max - zoomFactor }));
  }, [uPlotInst, startTouchDistance, xRange]);

  useEventListener("keydown", handleKeyDown);
  useEventListener("touchmove", handleTouchMove);
  useEventListener("touchstart", handleTouchStart);

  return null;
};

export default useZoomChart;
