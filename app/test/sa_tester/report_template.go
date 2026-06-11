package main

// reportPageTemplate is the HTML skeleton for GET /report.
// All __TOKEN__ placeholders are replaced at render time by handleReport.
var reportPageTemplate = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8">
<title>SA Tester Report</title>
<style>
body{font-family:'Segoe UI',Arial,sans-serif;margin:0;padding:20px;background:#f0f2f5;color:#333}
h1{margin-bottom:4px}
.subtitle{color:#666;font-size:14px;margin-bottom:24px}
.card{background:#fff;padding:20px;margin:16px 0;border-radius:8px;box-shadow:0 1px 4px rgba(0,0,0,.12)}
h2{margin:0 0 12px;color:#444;font-size:18px;border-bottom:1px solid #eee;padding-bottom:8px}
h3{margin:4px 0 8px;font-size:13px;font-family:monospace;color:#555;word-break:break-all}
pre{background:#f7f7f7;padding:12px;border-radius:4px;overflow-x:auto;font-size:13px;margin:0}
.chart-wrap{position:relative;height:340px;margin-bottom:24px}
.no-data{color:#aaa;font-style:italic;font-size:14px}
a{color:#1a73e8}
</style>
</head>
<body>
<h1>SA Tester Report</h1>
<p class="subtitle">Generated: __GENERATED_AT__ &nbsp;|&nbsp; <a href="/report">Refresh</a></p>
<div class="card">
  <h2>SA Config</h2>
  <pre>__SA_YAML__</pre>
</div>
<div class="card">
  <h2>Sent Series (__SENT_COUNT__ series)</h2>
  __SENT_CANVAS__
</div>
<div class="card">
  <h2>Received Series / SA Output (__RECV_COUNT__ series)</h2>
  __RECV_CHARTS__
</div>
<script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.min.js"></script>
<script>
(function(){
  var SENT = __SENT_JSON__;
  var RECV = __RECV_JSON__;

  var PALETTE = [
    'rgba(54,162,235,0.85)',
    'rgba(255,99,132,0.85)',
    'rgba(153,102,255,0.85)',
    'rgba(255,205,86,0.9)',
    'rgba(75,192,192,0.85)',
  ];

  function fmtDate(d) {
    return d.getUTCFullYear() + '-' +
           (d.getUTCMonth()+1).toString().padStart(2,'0') + '-' +
           d.getUTCDate().toString().padStart(2,'0');
  }
  function fmtTime(d) {
    return d.getUTCHours().toString().padStart(2,'0') + ':' +
           d.getUTCMinutes().toString().padStart(2,'0') + ':' +
           d.getUTCSeconds().toString().padStart(2,'0');
  }
  // Tooltip: full datetime with milliseconds.
  function fmtTs(sec) {
    var d = new Date(sec * 1000);
    return fmtDate(d) + ' ' + fmtTime(d) + '.' +
           d.getUTCMilliseconds().toString().padStart(3,'0');
  }
  // Tick label: show date only when it changes (or on the first tick).
  function fmtTick(v, idx, ticks) {
    var d = new Date(v * 1000);
    var dateStr = fmtDate(d);
    var timeStr = fmtTime(d);
    if (idx === 0) { return timeStr + ' ' + dateStr; }
    var prev = new Date(ticks[idx - 1].value * 1000);
    if (fmtDate(prev) !== dateStr) { return timeStr + ' ' + dateStr; }
    return timeStr;
  }

  // Plugin: draw dashed delay-span lines directly on the canvas so they are
  // owned by their series dataset and toggle with it.
  var delaySpanPlugin = {
    id: 'delaySpan',
    afterDatasetsDraw: function(chart) {
      var ctx2 = chart.ctx;
      chart.data.datasets.forEach(function(ds, dsi) {
        if (!chart.isDatasetVisible(dsi)) return;
        var meta = chart.getDatasetMeta(dsi);
        ds.data.forEach(function(pt, pi) {
          if (!pt || !pt.delayed) return;
          var el = meta.data[pi];
          if (!el) return;
          var xScale = chart.scales.x;
          var yScale = chart.scales.y;
          var x1 = el.x;
          var x2 = xScale.getPixelForValue(pt.sentAt);
          var y  = el.y;
          ctx2.save();
          ctx2.beginPath();
          ctx2.setLineDash([5, 4]);
          ctx2.strokeStyle = 'rgba(255,159,64,0.7)';
          ctx2.lineWidth = 2;
          ctx2.moveTo(x1, y);
          ctx2.lineTo(x2, y);
          ctx2.stroke();
          // Arrow head at sent-at end.
          var dir = x2 > x1 ? 1 : -1;
          ctx2.setLineDash([]);
          ctx2.beginPath();
          ctx2.moveTo(x2, y);
          ctx2.lineTo(x2 - dir * 8, y - 5);
          ctx2.lineTo(x2 - dir * 8, y + 5);
          ctx2.closePath();
          ctx2.fillStyle = 'rgba(255,159,64,0.7)';
          ctx2.fill();
          ctx2.restore();
        });
      });
    },
  };

  // x-axis always starts from when /start was called.
  var START_TS = __START_TS__; // unix seconds, injected by Go

  // Compute global x range: min is pinned to START_TS; max comes from data.
  var xMax = -Infinity;
  SENT.forEach(function(s) {
    s.points.forEach(function(p) {
      if (!p) return;
      if (p.x    > xMax) xMax = p.x;
      if (p.sentAt !== undefined && p.sentAt > xMax) xMax = p.sentAt;
    });
  });
  RECV.forEach(function(s) {
    s.points.forEach(function(p) {
      if (!p) return;
      if (p.x > xMax) xMax = p.x;
    });
  });
  // Add a 5 % right-side margin; left side is pinned exactly to START_TS.
  var xPad = xMax === -Infinity ? 1 : (xMax - START_TS) * 0.05 || 1;
  var xRange = { min: START_TS, max: (xMax === -Infinity ? START_TS + 60 : xMax + xPad) };

  var sentEl = document.getElementById('sent-all');
  if (sentEl && SENT.length > 0) {
    var datasets = [];
    SENT.forEach(function(s, si) {
      var col = PALETTE[si % PALETTE.length];
      var delayCol = 'rgba(255,159,64,0.9)';
      var pts = s.points.filter(function(p){ return p !== null; });
      if (pts.length === 0) return;
      // Attach full metadata (delayed, sentAt) to each chart point so the
      // plugin and tooltip can read it without separate datasets.
      datasets.push({
        type: 'scatter', label: s.name,
        data: pts.map(function(p){
          return {x: p.x, y: p.y, delayed: p.delayed, sentAt: p.sentAt};
        }),
        backgroundColor: pts.map(function(p){ return p.delayed ? delayCol : col; }),
        pointStyle:      pts.map(function(p){ return p.delayed ? 'triangle' : 'circle'; }),
        pointRadius:     pts.map(function(p){ return p.delayed ? 9 : 7; }),
      });
    });
    new Chart(sentEl, {
      type: 'scatter',
      data: { datasets: datasets },
      plugins: [delaySpanPlugin],
      options: {
        responsive: true, maintainAspectRatio: false,
        plugins: {
          legend: {
            position: 'bottom',
            onClick: function(e, legendItem, legend) {
              var chart = legend.chart;
              var idx = legendItem.datasetIndex;
              var meta = chart.getDatasetMeta(idx);
              var allOthersHidden = chart.data.datasets.every(function(_, i) {
                return i === idx || !chart.isDatasetVisible(i);
              });
              if (allOthersHidden && !meta.hidden) {
                // Already solo — restore all.
                chart.data.datasets.forEach(function(_, i) { chart.show(i); });
              } else {
                // Solo this series.
                chart.data.datasets.forEach(function(_, i) { chart.hide(i); });
                chart.show(idx);
              }
            },
          },
          tooltip: { callbacks: { label: function(ctx){
            var p = ctx.raw;
            if (!p) return '';
            var msg = 'value=' + p.y + '  ts=' + fmtTs(p.x);
            if (p.delayed) {
              var delaySec = Math.round((p.sentAt - p.x) * 10) / 10;
              msg += '  \u2192 sent at ' + fmtTs(p.sentAt) + ' (+' + delaySec + 's)';
            }
            return msg;
          }}},
        },
        scales: {
          x: Object.assign({ type: 'linear',
               title: { display: true, text: 'Unix timestamp (seconds UTC)' },
               ticks: { callback: function(v, idx, ticks) { return fmtTick(v, idx, ticks); }, maxRotation: 35, minRotation: 35 } },
            xRange),
          y: { title: { display: true, text: 'value' } },
        },
      },
    });
  }

  // Plugin: highlight all received points that share the hovered timestamp.
  var sameTimestampPlugin = {
    id: 'sameTimestamp',
    afterDraw: function(chart) {
      if (!chart._hoveredTs) return;
      var ctx2 = chart.ctx;
      var ts = chart._hoveredTs;
      chart.data.datasets.forEach(function(ds, dsi) {
        var meta = chart.getDatasetMeta(dsi);
        ds.data.forEach(function(pt, pi) {
          if (!pt || pt.x !== ts) return;
          var el = meta.data[pi];
          if (!el) return;
          ctx2.save();
          ctx2.beginPath();
          ctx2.arc(el.x, el.y, (el.options.radius || 6) + 5, 0, 2 * Math.PI);
          ctx2.strokeStyle = 'rgba(75,192,75,0.9)';
          ctx2.lineWidth = 2;
          ctx2.stroke();
          ctx2.restore();
        });
      });
    },
  };

  RECV.forEach(function(s, i) {
    var el = document.getElementById('recv-' + i);
    if (!el) return;
    var pts = s.points
      .filter(function(p){ return p !== null && p !== undefined && typeof p.x === 'number'; })
      .sort(function(a, b){ return a.x - b.x; });
    var recvChart = new Chart(el, {
      type: 'scatter',
      data: { datasets: [{
        label: s.name, data: pts,
        backgroundColor: 'rgba(75,192,75,0.85)',
        pointRadius: 6, pointStyle: 'circle',
      }]},
      plugins: [sameTimestampPlugin],
      options: {
        responsive: true, maintainAspectRatio: false,
        plugins: {
          legend: { display: false },
          tooltip: { callbacks: { label: function(ctx){
            var p = ctx.raw;
            if (p === null || p === undefined) return '';
            return 'value=' + p.y + '  ts=' + fmtTs(p.x);
          }}},
        },
        scales: {
          x: Object.assign({ type: 'linear',
               title: { display: true, text: 'Unix timestamp (seconds UTC)' },
               ticks: { callback: function(v, idx, ticks) { return fmtTick(v, idx, ticks); }, maxRotation: 35, minRotation: 35 } },
            xRange),
          y: { title: { display: true, text: 'value' } },
        },
        onHover: function(evt, active) {
          var ts = (active && active.length > 0)
            ? recvChart.data.datasets[active[0].datasetIndex].data[active[0].index].x
            : null;
          if (recvChart._hoveredTs !== ts) {
            recvChart._hoveredTs = ts;
            recvChart.draw();
          }
        },
      },
    });
  });
})();
</script>
</body>
</html>`
