import uPlot, { OrientCallback } from "uplot";

const deg360 = 2 * Math.PI;

export const drawPoints = (u: uPlot, seriesIdx: number) => {
  const size = 4 * uPlot.pxRatio;
  const r = size / 2;
  const squareSize = size * 1.2;
  const squareHalf = squareSize / 2;

  const orientCallback: OrientCallback = (
    series,
    dataX,
    dataY,
    scaleX,
    scaleY,
    valToPosX,
    valToPosY,
    xOff,
    yOff,
    xDim,
    yDim,
    _moveTo,
    _lineTo,
    rect,
    arc,
  ) => {
    const stroke = series?.stroke as unknown;
    if (typeof stroke === "function") {
      u.ctx.fillStyle = (stroke as () => string)();
    }

    const circlesPath = new Path2D();
    const squaresPath = new Path2D();

    const xMin = Number(scaleX.min);
    const xMax = Number(scaleX.max);
    const yMin = Number(scaleY.min);
    const yMax = Number(scaleY.max);

    const counts = new Map<string, number>();
    const len = dataX.length;

    for (let i = 0; i < len; i++) {
      const xv = dataX[i];
      const yv = dataY[i];

      if (xv == null || yv == null) continue;

      const xVal = Number(xv);
      const yVal = Number(yv);

      if (!Number.isFinite(xVal) || !Number.isFinite(yVal)) continue;

      const key = `${xVal}|${yVal}`;
      counts.set(key, (counts.get(key) ?? 0) + 1);
    }

    const duplicates = new Set<string>();
    for (const [key, count] of counts) {
      if (count > 1) duplicates.add(key);
    }

    for (let i = 0; i < len; i++) {
      const xv = dataX[i];
      const yv = dataY[i];

      if (xv == null || yv == null) continue;

      const xVal = Number(xv);
      const yVal = Number(yv);

      if (
        !Number.isFinite(xVal) ||
        !Number.isFinite(yVal) ||
        xVal < xMin || xVal > xMax ||
        yVal < yMin || yVal > yMax
      ) {
        continue;
      }

      const cx = valToPosX(xVal, scaleX, xDim, xOff);
      const cy = valToPosY(yVal, scaleY, yDim, yOff);

      const key = `${xVal}|${yVal}`;
      const isDuplicate = duplicates.has(key);

      if (isDuplicate) {
        rect(squaresPath, cx - squareHalf, cy - squareHalf, squareSize, squareSize);
      } else {
        circlesPath.moveTo(cx + r, cy);
        arc(circlesPath, cx, cy, r, 0, deg360);
      }
    }

    u.ctx.fill(circlesPath);
    u.ctx.lineWidth = 1.4 * uPlot.pxRatio;
    u.ctx.strokeStyle = u.ctx.fillStyle;
    u.ctx.stroke(squaresPath);
  };

  uPlot.orient(u, seriesIdx, orientCallback);

  return null;
};
