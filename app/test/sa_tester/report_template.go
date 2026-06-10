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

  var sentEl = document.getElementById('sent-all');
  if (sentEl && SENT.length > 0) {
    var datasets = [];
    var delaySpanAdded = false;
    SENT.forEach(function(s, si) {
      var col = PALETTE[si % PALETTE.length];
      var delayCol = 'rgba(255,159,64,0.9)';
      var pts = s.points.filter(function(p){ return p !== null; });
      if (pts.length === 0) return;
      // One dataset per series — per-point styling marks delayed points as triangles.
      datasets.push({
        type: 'scatter', label: s.name,
        data: pts.map(function(p){ return {x: p.x, y: p.y}; }),
        backgroundColor: pts.map(function(p){ return p.delayed ? delayCol : col; }),
        pointStyle:      pts.map(function(p){ return p.delayed ? 'triangle' : 'circle'; }),
        pointRadius:     pts.map(function(p){ return p.delayed ? 9 : 7; }),
      });
      // One line dataset per delay span (dashed arrow from data-ts to sent-at).
      pts.filter(function(p){ return p.delayed; }).forEach(function(p) {
        datasets.push({
          type: 'line',
          label: !delaySpanAdded ? 'Delay span (data\u2192sent)' : '',
          data: [{x: p.x, y: p.y}, {x: p.sentAt, y: p.y}],
          borderColor: 'rgba(255,159,64,0.6)',
          borderDash: [5, 5], borderWidth: 2, pointRadius: 0, fill: false,
        });
        delaySpanAdded = true;
      });
    });
    new Chart(sentEl, {
      type: 'scatter',
      data: { datasets: datasets },
      options: {
        responsive: true, maintainAspectRatio: false,
        plugins: {
          legend: {
            position: 'bottom',
            labels: { filter: function(item) { return item.text !== ''; } },
            onClick: function(e, legendItem, legend) {
              var chart = legend.chart;
              var idx = legendItem.datasetIndex;
              var allHidden = chart.data.datasets.every(function(ds, i) {
                return i === idx || !chart.isDatasetVisible(i);
              });
              // If clicking the one already-solo item, show everything again.
              if (allHidden) {
                chart.data.datasets.forEach(function(_, i) { chart.show(i); });
              } else {
                // Hide all, then show only the selected dataset.
                chart.data.datasets.forEach(function(_, i) { chart.hide(i); });
                chart.show(idx);
              }
            },
          },
          tooltip: { callbacks: { label: function(ctx){
            var p = ctx.raw;
            if (p === null || p === undefined) return '';
            return 'value=' + p.y + '  ts=' + fmtTs(p.x);
          }}},
        },
        scales: {
          x: { type: 'linear',
               title: { display: true, text: 'Unix timestamp (seconds UTC)' },
               ticks: { callback: function(v, idx, ticks) { return fmtTick(v, idx, ticks); }, maxRotation: 35, minRotation: 35 } },
          y: { title: { display: true, text: 'value' } },
        },
      },
    });
  }

  RECV.forEach(function(s, i) {
    var el = document.getElementById('recv-' + i);
    if (!el) return;
    var pts = s.points
      .filter(function(p){ return p !== null && p !== undefined && typeof p.x === 'number'; })
      .sort(function(a, b){ return a.x - b.x; });
    new Chart(el, {
      type: 'scatter',
      data: { datasets: [{
        label: s.name, data: pts,
        backgroundColor: 'rgba(75,192,75,0.85)',
        pointRadius: 6, pointStyle: 'circle',
      }]},
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
          x: { type: 'linear',
               title: { display: true, text: 'Unix timestamp (seconds UTC)' },
               ticks: { callback: function(v, idx, ticks) { return fmtTick(v, idx, ticks); }, maxRotation: 35, minRotation: 35 } },
          y: { title: { display: true, text: 'value' } },
        },
      },
    });
  });
})();
</script>
</body>
</html>`
