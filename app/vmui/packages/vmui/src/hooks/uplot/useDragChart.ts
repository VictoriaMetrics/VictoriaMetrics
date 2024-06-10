import { useRef } from "preact/compat";
import uPlot from "uplot";
import { SetMinMax } from "../../types";

interface DragHookArgs {
  dragSpeed: number,
  setPanning: (enable: boolean) => void,
  setPlotScale: SetMinMax
}

interface DragArgs {
  e: MouseEvent | TouchEvent,
  u: uPlot,
}

const isMouseEvent = (e: MouseEvent | TouchEvent): e is MouseEvent => e instanceof MouseEvent;
const getClientX = (e: MouseEvent | TouchEvent) => isMouseEvent(e) ? e.clientX : e.touches[0].clientX;

const useDragChart = ({ dragSpeed = 0.85, setPanning, setPlotScale }: DragHookArgs) => {
  const dragState = useRef({
    leftStart: 0,
    xUnitsPerPx: 0,
    scXMin: 0,
    scXMax: 0,
  });

  const mouseMove = (e: MouseEvent | TouchEvent) => {
    e.preventDefault();
    const clientX = getClientX(e);
    const { leftStart, xUnitsPerPx, scXMin, scXMax } = dragState.current;
    const dx = xUnitsPerPx * ((clientX - leftStart) * dragSpeed);
    setPlotScale({ min: scXMin - dx, max: scXMax - dx });
  };

  const mouseUp = () => {
    setPanning(false);
    document.removeEventListener("mousemove", mouseMove);
    document.removeEventListener("mouseup", mouseUp);
    document.removeEventListener("touchmove", mouseMove);
    document.removeEventListener("touchend", mouseUp);
  };

  const mouseDown = () => {
    document.addEventListener("mousemove", mouseMove);
    document.addEventListener("mouseup", mouseUp);
    document.addEventListener("touchmove", mouseMove);
    document.addEventListener("touchend", mouseUp);
  };

  return ({ e, u }: DragArgs): void => {
    e.preventDefault();
    setPanning(true);

    dragState.current = {
      leftStart: getClientX(e),
      xUnitsPerPx: u.posToVal(1, "x") - u.posToVal(0, "x"),
      scXMin: u.scales.x.min || 0,
      scXMax: u.scales.x.max || 0,
    };

    mouseDown();
  };
};

export default useDragChart;
