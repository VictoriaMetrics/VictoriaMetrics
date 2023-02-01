/* eslint-disable */
import uPlot from "uplot";
import {getCssVariable} from "../theme";
import {sizeAxis} from "./helpers";

export const seriesBarsPlugin = (opts) => {
  let pxRatio;
  let font;

  let { ignore = [] } = opts;

  let radius = opts.radius ?? 0;

  function setPxRatio() {
    pxRatio = devicePixelRatio;
    font = Math.round(10 * pxRatio) + "px Arial";
  }

  setPxRatio();

  window.addEventListener("dppxchange", setPxRatio);

  const ori        = opts.ori;
  const dir        = opts.dir;
  const stacked    = opts.stacked;

  const groupWidth = 0.9;
  const groupDistr = SPACE_BETWEEN;

  const barWidth   = 1;
  const barDistr   = SPACE_BETWEEN;

  function distrTwo(groupCount, barCount, _groupWidth = groupWidth) {
    let out = Array.from({length: barCount}, () => ({
      offs: Array(groupCount).fill(0),
      size: Array(groupCount).fill(0),
    }));

    distr(groupCount, _groupWidth, groupDistr, null, (groupIdx, groupOffPct, groupDimPct) => {
      distr(barCount, barWidth, barDistr, null, (barIdx, barOffPct, barDimPct) => {
        out[barIdx].offs[groupIdx] = groupOffPct + (groupDimPct * barOffPct);
        out[barIdx].size[groupIdx] = groupDimPct * barDimPct;
      });
    });

    return out;
  }

  function distrOne(groupCount, barCount) {
    let out = Array.from({length: barCount}, () => ({
      offs: Array(groupCount).fill(0),
      size: Array(groupCount).fill(0),
    }));

    distr(groupCount, groupWidth, groupDistr, null, (groupIdx, groupOffPct, groupDimPct) => {
      distr(barCount, barWidth, barDistr, null, (barIdx) => {
        out[barIdx].offs[groupIdx] = groupOffPct;
        out[barIdx].size[groupIdx] = groupDimPct;
      });
    });

    return out;
  }

  let barsPctLayout;
  let barsColors;

  let barsBuilder = uPlot.paths.bars({
    radius,
    disp: {
      x0: {
        unit: 2,
        values: (u, seriesIdx) => barsPctLayout[seriesIdx].offs,
      },
      size: {
        unit: 2,
        values: (u, seriesIdx) => barsPctLayout[seriesIdx].size,
      },
      ...opts.disp,
    },
    each: (u, seriesIdx, dataIdx, lft, top, wid, hgt) => {
      // we get back raw canvas coords (included axes & padding). translate to the plotting area origin
      lft -= u.bbox.left;
      top -= u.bbox.top;
      qt.add({x: lft, y: top, w: wid, h: hgt, sidx: seriesIdx, didx: dataIdx});
    },
  });

  function drawPoints(u, sidx) {
    u.ctx.save();

    u.ctx.font         = font;
    u.ctx.fillStyle    = getCssVariable("color-text");

    uPlot.orient(u, sidx, (
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
      yDim) => {
      const _dir = dir * (ori === 0 ? 1 : -1);

      const wid = Math.round(barsPctLayout[sidx].size[0] * xDim);

      barsPctLayout[sidx].offs.forEach((offs, ix) => {
        if (dataY[ix] !== null) {
          let x0     = xDim * offs;
          let lft    = Math.round(xOff + (_dir === 1 ? x0 : xDim - x0 - wid));
          let barWid = Math.round(wid);

          let yPos = valToPosY(dataY[ix], scaleY, yDim, yOff);

          let x = ori === 0 ? Math.round(lft + barWid/2) : Math.round(yPos);
          let y = ori === 0 ? Math.round(yPos)           : Math.round(lft + barWid/2);

          u.ctx.textAlign    = ori === 0 ? "center" : dataY[ix] >= 0 ? "left" : "right";
          u.ctx.textBaseline = ori === 1 ? "middle" : dataY[ix] >= 0 ? "bottom" : "top";

          u.ctx.fillText(dataY[ix], x, y);
        }
      });
    });

    u.ctx.restore();
  }

  function range(u, dataMin, dataMax) {
    return [0, uPlot.rangeNum(0, dataMax, 0.05, true)[1]];
  }

  let qt;

  return {
    hooks: {
      drawClear: u => {
        qt = qt || new Quadtree(0, 0, u.bbox.width, u.bbox.height);

        qt.clear();

        // force-clear the path cache to cause drawBars() to rebuild new quadtree
        u.series.forEach(s => {
          s._paths = null;
        });

        if (stacked)
          barsPctLayout = [null].concat(distrOne(u.data.length - 1 - ignore.length, u.data[0].length));
        else if (u.series.length === 2)
          barsPctLayout = [null].concat(distrOne(u.data[0].length, 1));
        else
          barsPctLayout = [null].concat(distrTwo(u.data[0].length, u.data.length - 1 - ignore.length, u.data[0].length === 1 ? 1 : groupWidth));

        // TODO only do on setData, not every redraw
        if (opts.disp?.fill != null) {
          barsColors = [null];

          for (let i = 1; i < u.data.length; i++) {
            barsColors.push({
              fill: opts.disp.fill.values(u, i),
              stroke: opts.disp.stroke.values(u, i),
            });
          }
        }
      },
    },
    opts: (u, opts) => {
      const yScaleOpts = {
        range,
        ori: ori === 0 ? 1 : 0,
      };

      // hovered
      let hRect;

      uPlot.assign(opts, {
        select: {show: false},
        cursor: {
          x: false,
          y: false,
          dataIdx: (u, seriesIdx) => {
            if (seriesIdx === 1) {
              hRect = null;

              let cx = u.cursor.left * pxRatio;
              let cy = u.cursor.top * pxRatio;

              qt.get(cx, cy, 1, 1, o => {
                if (pointWithin(cx, cy, o.x, o.y, o.x + o.w, o.y + o.h))
                  hRect = o;
              });
            }

            return hRect && seriesIdx === hRect.sidx ? hRect.didx : null;
          },
          points: {
            // fill: "rgba(255,255,255, 0.3)",
            bbox: (u, seriesIdx) => {
              let isHovered = hRect && seriesIdx === hRect.sidx;

              return {
                left:   isHovered ? hRect.x / pxRatio : -10,
                top:    isHovered ? hRect.y / pxRatio : -10,
                width:  isHovered ? hRect.w / pxRatio : 0,
                height: isHovered ? hRect.h / pxRatio : 0,
              };
            }
          }
        },
        scales: {
          x: {
            time: false,
            distr: 2,
            ori,
            dir,
            //	auto: true,
            range: (u, min, max) => {
              min = 0;
              max = Math.max(1, u.data[0].length - 1);

              let pctOffset = 0;

              distr(u.data[0].length, groupWidth, groupDistr, 0, (di, lftPct, widPct) => {
                pctOffset = lftPct + widPct / 2;
              });

              let rn = max - min;

              if (pctOffset === 0.5)
                min -= rn;
              else {
                let upScale = 1 / (1 - pctOffset * 2);
                let offset = (upScale * rn - rn) / 2;

                min -= offset;
                max += offset;
              }

              return [min, max];
            }
          },
          rend:   yScaleOpts,
          size:   yScaleOpts,
          mem:    yScaleOpts,
          inter:  yScaleOpts,
          toggle: yScaleOpts,
        }
      });

      if (ori === 1) {
        opts.padding = [0, null, 0, null];
      }

      uPlot.assign(opts.axes[0], {
        splits: (u) => {
          const _dir = dir * (ori === 0 ? 1 : -1);
          let splits = u._data[0].slice();
          return _dir === 1 ? splits : splits.reverse();
        },
        values:     u => u.data[0],
        gap:        15,
        size:       sizeAxis,
        stroke: getCssVariable("color-text"),
        font: "10px Arial",
        labelSize:  20,
        grid:       {show: false},
        ticks:      {show: false},

        side:       ori === 0 ? 2 : 3,
      });

      opts.series.forEach((s, i) => {
        if (i > 0 && !ignore.includes(i)) {
          uPlot.assign(s, {
            //	pxAlign: false,
            //	stroke: "rgba(255,0,0,0.5)",
            paths: barsBuilder,
            points: {
              show: drawPoints
            }
          });
        }
      });
    }
  };
};

const roundDec = (val, dec) => {
  return Math.round(val * (dec = 10**dec)) / dec;
}

const SPACE_BETWEEN = 1;
const SPACE_AROUND  = 2;
const SPACE_EVENLY  = 3;

const coord = (i, offs, iwid, gap) => roundDec(offs + i * (iwid + gap), 6);

const distr = (numItems, sizeFactor, justify, onlyIdx, each) => {
  let space = 1 - sizeFactor;

  let gap =  (
    justify === SPACE_BETWEEN ? space / (numItems - 1) :
      justify === SPACE_AROUND  ? space / (numItems    ) :
        justify === SPACE_EVENLY  ? space / (numItems + 1) : 0
  );

  if (isNaN(gap) || gap === Infinity)
    gap = 0;

  let offs = (
    justify === SPACE_BETWEEN ? 0       :
      justify === SPACE_AROUND  ? gap / 2 :
        justify === SPACE_EVENLY  ? gap     : 0
  );

  let iwid = sizeFactor / numItems;
  let _iwid = roundDec(iwid, 6);

  if (onlyIdx == null) {
    for (let i = 0; i < numItems; i++)
      each(i, coord(i, offs, iwid, gap), _iwid);
  }
  else
    each(onlyIdx, coord(onlyIdx, offs, iwid, gap), _iwid);
}

const pointWithin = (px, py, rlft, rtop, rrgt, rbtm) => {
  return px >= rlft && px <= rrgt && py >= rtop && py <= rbtm;
}

const MAX_OBJECTS = 10;
const MAX_LEVELS  = 4;

function Quadtree(x, y, w, h, l) {
  let t = this;

  t.x = x;
  t.y = y;
  t.w = w;
  t.h = h;
  t.l = l || 0;
  t.o = [];
  t.q = null;
}

const proto = {
  split: function() {
    let t = this,
      x = t.x,
      y = t.y,
      w = t.w / 2,
      h = t.h / 2,
      l = t.l + 1;

    t.q = [
      // top right
      new Quadtree(x + w, y,     w, h, l),
      // top left
      new Quadtree(x,     y,     w, h, l),
      // bottom left
      new Quadtree(x,     y + h, w, h, l),
      // bottom right
      new Quadtree(x + w, y + h, w, h, l),
    ];
  },

  // invokes callback with index of each overlapping quad
  quads: function(x, y, w, h, cb) {
    let t            = this,
      q            = t.q,
      hzMid        = t.x + t.w / 2,
      vtMid        = t.y + t.h / 2,
      startIsNorth = y     < vtMid,
      startIsWest  = x     < hzMid,
      endIsEast    = x + w > hzMid,
      endIsSouth   = y + h > vtMid;

    // top-right quad
    startIsNorth && endIsEast && cb(q[0]);
    // top-left quad
    startIsWest && startIsNorth && cb(q[1]);
    // bottom-left quad
    startIsWest && endIsSouth && cb(q[2]);
    // bottom-right quad
    endIsEast && endIsSouth && cb(q[3]);
  },

  add: function(o) {
    let t = this;

    if (t.q != null) {
      t.quads(o.x, o.y, o.w, o.h, q => {
        q.add(o);
      });
    }
    else {
      let os = t.o;

      os.push(o);

      if (os.length > MAX_OBJECTS && t.l < MAX_LEVELS) {
        t.split();

        for (let i = 0; i < os.length; i++) {
          let oi = os[i];

          t.quads(oi.x, oi.y, oi.w, oi.h, q => {
            q.add(oi);
          });
        }

        t.o.length = 0;
      }
    }
  },

  get: function(x, y, w, h, cb) {
    let t = this;
    let os = t.o;

    for (let i = 0; i < os.length; i++)
      cb(os[i]);

    if (t.q != null) {
      t.quads(x, y, w, h, q => {
        q.get(x, y, w, h, cb);
      });
    }
  },

  clear: function() {
    this.o.length = 0;
    this.q = null;
  },
};

Object.assign(Quadtree.prototype, proto);

global.Quadtree = Quadtree;
