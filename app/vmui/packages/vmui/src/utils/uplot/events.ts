import {DragArgs} from "./types";

export const dragChart = ({e, factor = 0.85, u, setPanning, setPlotScale}: DragArgs): void => {
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

export const zoomChart = ({e, factor = 0.85, u, setPanning, setPlotScale}: DragArgs): void => {
  setPanning(true);
  e.preventDefault();
  const {left = 0} = u.cursor;
  const rect = u.over.getBoundingClientRect();
  const scXMin = u.scales.x.min || 0;
  const scXMax = u.scales.x.max || 0;
  const xVal = u.posToVal(left, "x");
  const leftPct = left/rect.width;
  const xUnitsPerPx = u.posToVal(1, "x") - u.posToVal(0, "x");

  const mouseMove = (e: MouseEvent) => {
    e.preventDefault();
    const dx = xUnitsPerPx * (e.clientX - left) * factor;
    const nxRange = scXMax - scXMin - dx;
    const min = xVal - leftPct * nxRange;
    const max = min + nxRange;
    setPlotScale({u, min, max});
  };

  const mouseUp = () => {
    setPanning(false);
    document.removeEventListener("mousemove", mouseMove);
    document.removeEventListener("mouseup", mouseUp);
  };

  document.addEventListener("mousemove", mouseMove);
  document.addEventListener("mouseup", mouseUp);
};
