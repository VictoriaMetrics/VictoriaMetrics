import {DragArgs} from "./types";

export const dragChart = ({e, factor = 0.85, u, setPanning, setPlotScale}: DragArgs): void => {
  if (e.button !== 0) return;
  e.preventDefault();
  setPanning(true);
  const leftStart = e.clientX;
  const xUnitsPerPx = u.posToVal(1, "x") - u.posToVal(0, "x");
  const scXMin = u.scales.x.min || 0;
  const scXMax = u.scales.x.max || 0;

  const mouseMove = (e: MouseEvent) => {
    e.preventDefault();
    const dx = xUnitsPerPx * ((e.clientX - leftStart) * factor);
    setPlotScale({u, min: scXMin - dx, max: scXMax - dx});
  };
  const mouseUp = () => {
    setPanning(false);
    document.removeEventListener("mousemove", mouseMove);
    document.removeEventListener("mouseup", mouseUp);
  };

  document.addEventListener("mousemove", mouseMove);
  document.addEventListener("mouseup", mouseUp);
};