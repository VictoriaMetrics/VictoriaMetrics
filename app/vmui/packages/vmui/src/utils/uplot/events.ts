import { DragArgs } from "./types";

export const dragChart = ({ e, factor = 0.85, u, setPanning, setPlotScale }: DragArgs): void => {
  e.preventDefault();
  const isMouseEvent = e instanceof MouseEvent;

  setPanning(true);
  const leftStart = isMouseEvent ? e.clientX : e.touches[0].clientX;
  const xUnitsPerPx = u.posToVal(1, "x") - u.posToVal(0, "x");
  const scXMin = u.scales.x.min || 0;
  const scXMax = u.scales.x.max || 0;

  const mouseMove = (e: MouseEvent | TouchEvent) => {
    const isMouseEvent = e instanceof MouseEvent;
    if (!isMouseEvent && e.touches.length > 1) return;
    e.preventDefault();

    const clientX = isMouseEvent ? e.clientX : e.touches[0].clientX;
    const dx = xUnitsPerPx * ((clientX - leftStart) * factor);
    setPlotScale({ min: scXMin - dx, max: scXMax - dx });
  };
  const mouseUp = () => {
    setPanning(false);
    document.removeEventListener("mousemove", mouseMove);
    document.removeEventListener("mouseup", mouseUp);
    document.removeEventListener("touchmove", mouseMove);
    document.removeEventListener("touchend", mouseUp);
  };

  document.addEventListener("mousemove", mouseMove);
  document.addEventListener("mouseup", mouseUp);
  document.addEventListener("touchmove", mouseMove);
  document.addEventListener("touchend", mouseUp);
};
