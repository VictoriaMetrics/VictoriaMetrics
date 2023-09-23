import React from "preact/compat";
import { FunctionIcon } from "../components/Main/Icons";

const docsUrl = "https://docs.victoriametrics.com/MetricsQL.html";

export default [
  {
    value: "absent_over_time",
    description: `<code>absent_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which returns 1\n  if the given lookbehind window <code>d</code> doesn't contain raw samples. Otherwise, it returns an empty result.`,
    type: "Rollup function"
  },
  {
    value: "aggr_over_time",
    description: `<code>aggr_over_time(("rollup_func1", "rollup_func2", ...), series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>,\n  which calculates all the listed <code>rollup_func*</code> for raw samples on the given lookbehind window <code>d</code>.\n  The calculations are performed individually per each time series returned\n  from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "ascent_over_time",
    description: `<code>ascent_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates\n  ascent of raw sample values on the given lookbehind window <code>d</code>. The calculations are performed individually\n  per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "avg_over_time",
    description: `<code>avg_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the average value\n  over raw samples on the given lookbehind window <code>d</code> per each time series returned\n  from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "changes",
    description: `<code>changes(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the number of times\n  the raw samples changed on the given lookbehind window <code>d</code> per each time series returned\n  from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "changes_prometheus",
    description: `<code>changes_prometheus(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the number of times\n  the raw samples changed on the given lookbehind window <code>d</code> per each time series returned\n  from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "count_eq_over_time",
    description: `<code>count_eq_over_time(series_selector[d], eq)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the number of raw samples\n  on the given lookbehind window <code>d</code>, which are equal to <code>eq</code>. It is calculated independently per each time series returned\n  from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "count_gt_over_time",
    description: `<code>count_gt_over_time(series_selector[d], gt)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the number of raw samples\n  on the given lookbehind window <code>d</code>, which are bigger than <code>gt</code>. It is calculated independently per each time series returned\n  from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "count_le_over_time",
    description: `<code>count_le_over_time(series_selector[d], le)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the number of raw samples\n  on the given lookbehind window <code>d</code>, which don't exceed <code>le</code>. It is calculated independently per each time series returned\n  from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "count_ne_over_time",
    description: `<code>count_ne_over_time(series_selector[d], ne)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the number of raw samples\n  on the given lookbehind window <code>d</code>, which aren't equal to <code>ne</code>. It is calculated independently per each time series returned\n  from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "count_over_time",
    description: `<code>count_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the number of raw samples\n  on the given lookbehind window <code>d</code> per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "decreases_over_time",
    description: `<code>decreases_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the number of raw sample value decreases\n  over the given lookbehind window <code>d</code> per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "default_rollup",
    description: `<code>default_rollup(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which returns the last raw sample value on the given lookbehind window <code>d</code>\n  per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "delta",
    description: `<code>delta(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the difference between\n  the last sample before the given lookbehind window <code>d</code> and the last sample at the given lookbehind window <code>d</code>\n  per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "delta_prometheus",
    description: `<code>delta_prometheus(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the difference between\n  the first and the last samples at the given lookbehind window <code>d</code> per each time series returned\n  from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "deriv",
    description: `<code>deriv(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates per-second derivative over the given lookbehind window <code>d</code>\n  per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.\n  The derivative is calculated using linear regression.`,
    type: "Rollup function"
  },
  {
    value: "deriv_fast",
    description: `<code>deriv_fast(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates per-second derivative\n  using the first and the last raw samples on the given lookbehind window <code>d</code> per each time series returned\n  from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "descent_over_time",
    description: `<code>descent_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates descent of raw sample values\n  on the given lookbehind window <code>d</code>. The calculations are performed individually per each time series returned\n  from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "distinct_over_time",
    description: `<code>distinct_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which returns the number of distinct raw sample values\n  on the given lookbehind window <code>d</code> per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "duration_over_time",
    description: `<code>duration_over_time(series_selector[d], max_interval)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which returns the duration in seconds\n  when time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a> were present\n  over the given lookbehind window <code>d</code>. It is expected that intervals between adjacent samples per each series don't exceed the <code>max_interval</code>.\n  Otherwise, such intervals are considered as gaps and aren't counted.`,
    type: "Rollup function"
  },
  {
    value: "first_over_time",
    description: `<code>first_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which returns the first raw sample value\n  on the given lookbehind window <code>d</code> per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "geomean_over_time",
    description: `<code>geomean_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates <a class="vm-link vm-link_colored" target="_blank" href="https://en.wikipedia.org/wiki/Geometric_mean">geometric mean</a>\n  over raw samples on the given lookbehind window <code>d</code> per each time series returned\n  from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "histogram_over_time",
    description: `<code>histogram_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates\n  <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}https://godoc.org/github.com/VictoriaMetrics/metrics#Histogram">VictoriaMetrics histogram</a> over raw samples on the given lookbehind window <code>d</code>.\n  It is calculated individually per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.\n  The resulting histograms are useful to pass to <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#histogram_quantile">histogram_quantile</a> for calculating quantiles\n  over multiple <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#gauge">gauges</a>.\n  For example, the following query calculates median temperature by country over the last 24 hours:`,
    type: "Rollup function"
  },
  {
    value: "hoeffding_bound_lower",
    description: `<code>hoeffding_bound_lower(phi, series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates\n  lower <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}https://en.wikipedia.org/wiki/Hoeffding%27s_inequality">Hoeffding bound</a> for the given <code>phi</code> in the range <code>[0...1]</code>.`,
    type: "Rollup function"
  },
  {
    value: "hoeffding_bound_upper",
    description: `<code>hoeffding_bound_upper(phi, series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates\n  upper <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}https://en.wikipedia.org/wiki/Hoeffding%27s_inequality">Hoeffding bound</a> for the given <code>phi</code> in the range <code>[0...1]</code>.`,
    type: "Rollup function"
  },
  {
    value: "holt_winters",
    description: `<code>holt_winters(series_selector[d], sf, tf)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates Holt-Winters value\n  (aka <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}https://en.wikipedia.org/wiki/Exponential_smoothing#Double_exponential_smoothing">double exponential smoothing</a>) for raw samples\n  over the given lookbehind window <code>d</code> using the given smoothing factor <code>sf</code> and the given trend factor <code>tf</code>.\n  Both <code>sf</code> and <code>tf</code> must be in the range <code>[0...1]</code>. It is expected that the <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>\n  returns time series of <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#gauge">gauge type</a>.`,
    type: "Rollup function"
  },
  {
    value: "idelta",
    description: `<code>idelta(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the difference between the last two raw samples\n  on the given lookbehind window <code>d</code> per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "ideriv",
    description: `<code>ideriv(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the per-second derivative based on the last two raw samples\n  over the given lookbehind window <code>d</code>. The derivative is calculated independently per each time series returned\n  from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "increase",
    description: `<code>increase(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the increase over the given lookbehind window <code>d</code>\n  per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.\n  It is expected that the <code>series_selector</code> returns time series of <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#counter">counter type</a>.`,
    type: "Rollup function"
  },
  {
    value: "increase_prometheus",
    description: `<code>increase_prometheus(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the increase\n  over the given lookbehind window <code>d</code> per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.\n  It is expected that the <code>series_selector</code> returns time series of <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#counter">counter type</a>.\n  It doesn't take into account the last sample before the given lookbehind window <code>d</code> when calculating the result in the same way as Prometheus does.\n  See <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}https://medium.com/@romanhavronenko/victoriametrics-promql-compliance-d4318203f51e">this article</a> for details.`,
    type: "Rollup function"
  },
  {
    value: "increase_pure",
    description: `<code>increase_pure(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which works the same as <a class="vm-link vm-link_colored" target="_blank" href="#increase">increase</a> except\n  of the following corner case - it assumes that <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#counter">counters</a> always start from 0,\n  while <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#increase">increase</a> ignores the first value in a series if it is too big.`,
    type: "Rollup function"
  },
  {
    value: "increases_over_time",
    description: `<code>increases_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the number of raw sample value increases\n  over the given lookbehind window <code>d</code> per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "integrate",
    description: `<code>integrate(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the integral over raw samples on the given lookbehind window <code>d</code>\n  per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "irate",
    description: `<code>irate(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the "instant" per-second increase rate over the last two raw samples\n  on the given lookbehind window <code>d</code> per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.\n  It is expected that the <code>series_selector</code> returns time series of <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#counter">counter type</a>.`,
    type: "Rollup function"
  },
  {
    value: "lag",
    description: `<code>lag(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which returns the duration in seconds between the last sample\n  on the given lookbehind window <code>d</code> and the timestamp of the current point. It is calculated independently per each time series returned\n  from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "last_over_time",
    description: `<code>last_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which returns the last raw sample value on the given lookbehind window <code>d</code>\n  per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "lifetime",
    description: `<code>lifetime(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which returns the duration in seconds between the last and the first sample\n  on the given lookbehind window <code>d</code> per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "mad_over_time",
    description: `<code>mad_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates <a class="vm-link vm-link_colored" target="_blank" href="https://en.wikipedia.org/wiki/Median_absolute_deviation">median absolute deviation</a>\n  over raw samples on the given lookbehind window <code>d</code> per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "max_over_time",
    description: `<code>max_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the maximum value over raw samples\n  on the given lookbehind window <code>d</code> per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "median_over_time",
    description: `<code>median_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates median value over raw samples\n  on the given lookbehind window <code>d</code> per each time series returned\n  from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "min_over_time",
    description: `<code>min_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the minimum value over raw samples\n  on the given lookbehind window <code>d</code> per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "mode_over_time",
    description: `<code>mode_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates <a class="vm-link vm-link_colored" target="_blank" href="https://en.wikipedia.org/wiki/Mode_(statistics">mode</a>)\n  for raw samples on the given lookbehind window <code>d</code>. It is calculated individually per each time series returned\n  from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>. It is expected that raw sample values are discrete.`,
    type: "Rollup function"
  },
  {
    value: "predict_linear",
    description: `<code>predict_linear(series_selector[d], t)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the value <code>t</code> seconds in the future using\n  linear interpolation over raw samples on the given lookbehind window <code>d</code>. The predicted value is calculated individually per each time series\n  returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "present_over_time",
    description: `<code>present_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which returns 1 if there is at least a single raw sample\n  on the given lookbehind window <code>d</code>. Otherwise, an empty result is returned.`,
    type: "Rollup function"
  },
  {
    value: "quantile_over_time",
    description: `<code>quantile_over_time(phi, series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates <code>phi</code>-quantile over raw samples\n  on the given lookbehind window <code>d</code> per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.\n  The <code>phi</code> value must be in the range <code>[0...1]</code>.`,
    type: "Rollup function"
  },
  {
    value: "quantiles_over_time",
    description: `<code>quantiles_over_time("phiLabel", phi1, ..., phiN, series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates <code>phi*</code>-quantiles\n  over raw samples on the given lookbehind window <code>d</code> per each time series returned\n  from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.\n  The function returns individual series per each <code>phi*</code> with <code>{phiLabel="phi*"}</code> label. <code>phi*</code> values must be in the range <code>[0...1]</code>.`,
    type: "Rollup function"
  },
  {
    value: "range_over_time",
    description: `<code>range_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates value range over raw samples\n  on the given lookbehind window <code>d</code> per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.\n  E.g. it calculates <code>max_over_time(series_selector[d]) - min_over_time(series_selector[d])</code>.`,
    type: "Rollup function"
  },
  {
    value: "rate",
    description: `<code>rate(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the average per-second increase rate\n  over the given lookbehind window <code>d</code> per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.\n  It is expected that the <code>series_selector</code> returns time series of <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#counter">counter type</a>.`,
    type: "Rollup function"
  },
  {
    value: "rate_over_sum",
    description: `<code>rate_over_sum(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates per-second rate over the sum of raw samples\n  on the given lookbehind window <code>d</code>. The calculations are performed individually per each time series returned\n  from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "resets",
    description: `<code>resets(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which returns the number\n  of <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#counter">counter</a> resets over the given lookbehind window <code>d</code>\n  per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.\n  It is expected that the <code>series_selector</code> returns time series of <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#counter">counter type</a>.`,
    type: "Rollup function"
  },
  {
    value: "rollup",
    description: `<code>rollup(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates <code>min</code>, <code>max</code> and <code>avg</code> values for raw samples\n  on the given lookbehind window <code>d</code> and returns them in time series with <code>rollup="min"</code>, <code>rollup="max"</code> and <code>rollup="avg"</code> additional labels.\n  These values are calculated individually per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "rollup_candlestick",
    description: `<code>rollup_candlestick(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates <code>open</code>, <code>high</code>, <code>low</code> and <code>close</code> values (aka OHLC)\n  over raw samples on the given lookbehind window <code>d</code> and returns them in time series with <code>rollup="open"</code>, <code>rollup="high"</code>, <code>rollup="low"</code> and <code>rollup="close"</code> additional labels.\n  The calculations are performed individually per each time series returned\n  from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>. This function is useful for financial applications.`,
    type: "Rollup function"
  },
  {
    value: "rollup_delta",
    description: `<code>rollup_delta(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates differences between adjacent raw samples\n  on the given lookbehind window <code>d</code> and returns <code>min</code>, <code>max</code> and <code>avg</code> values for the calculated differences\n  and returns them in time series with <code>rollup="min"</code>, <code>rollup="max"</code> and <code>rollup="avg"</code> additional labels.\n  The calculations are performed individually per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "rollup_deriv",
    description: `<code>rollup_deriv(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates per-second derivatives\n  for adjacent raw samples on the given lookbehind window <code>d</code> and returns <code>min</code>, <code>max</code> and <code>avg</code> values for the calculated per-second derivatives\n  and returns them in time series with <code>rollup="min"</code>, <code>rollup="max"</code> and <code>rollup="avg"</code> additional labels.\n  The calculations are performed individually per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "rollup_increase",
    description: `<code>rollup_increase(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates increases for adjacent raw samples\n  on the given lookbehind window <code>d</code> and returns <code>min</code>, <code>max</code> and <code>avg</code> values for the calculated increases\n  and returns them in time series with <code>rollup="min"</code>, <code>rollup="max"</code> and <code>rollup="avg"</code> additional labels.\n  The calculations are performed individually per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "rollup_rate",
    description: `<code>rollup_rate(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates per-second change rates for adjacent raw samples\n  on the given lookbehind window <code>d</code> and returns <code>min</code>, <code>max</code> and <code>avg</code> values for the calculated per-second change rates\n  and returns them in time series with <code>rollup="min"</code>, <code>rollup="max"</code> and <code>rollup="avg"</code> additional labels.`,
    type: "Rollup function"
  },
  {
    value: "rollup_scrape_interval",
    description: `<code>rollup_scrape_interval(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the interval in seconds between\n  adjacent raw samples on the given lookbehind window <code>d</code> and returns <code>min</code>, <code>max</code> and <code>avg</code> values for the calculated interval\n  and returns them in time series with <code>rollup="min"</code>, <code>rollup="max"</code> and <code>rollup="avg"</code> additional labels.\n  The calculations are performed individually per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "scrape_interval",
    description: `<code>scrape_interval(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the average interval in seconds between raw samples\n  on the given lookbehind window <code>d</code> per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "share_gt_over_time",
    description: `<code>share_gt_over_time(series_selector[d], gt)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which returns share (in the range <code>[0...1]</code>) of raw samples\n  on the given lookbehind window <code>d</code>, which are bigger than <code>gt</code>. It is calculated independently per each time series returned\n  from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "share_le_over_time",
    description: `<code>share_le_over_time(series_selector[d], le)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which returns share (in the range <code>[0...1]</code>) of raw samples\n  on the given lookbehind window <code>d</code>, which are smaller or equal to <code>le</code>. It is calculated independently per each time series returned\n  from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "share_eq_over_time",
    description: `<code>share_eq_over_time(series_selector[d], eq)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which returns share (in the range <code>[0...1]</code>) of raw samples\n  on the given lookbehind window <code>d</code>, which are equal to <code>eq</code>. It is calculated independently per each time series returned\n  from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "stale_samples_over_time",
    description: `<code>stale_samples_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the number\n  of <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/vmagent.html#prometheus-staleness-markers">staleness markers</a> on the given lookbehind window <code>d</code>\n  per each time series matching the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "stddev_over_time",
    description: `<code>stddev_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates standard deviation over raw samples\n  on the given lookbehind window <code>d</code> per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "stdvar_over_time",
    description: `<code>stdvar_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates standard variance over raw samples\n  on the given lookbehind window <code>d</code> per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "sum_over_time",
    description: `<code>sum_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the sum of raw sample values\n  on the given lookbehind window <code>d</code> per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "sum2_over_time",
    description: `<code>sum2_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which calculates the sum of squares for raw sample values\n  on the given lookbehind window <code>d</code> per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "timestamp",
    description: `<code>timestamp(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which returns the timestamp in seconds for the last raw sample\n  on the given lookbehind window <code>d</code> per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "timestamp_with_name",
    description: `<code>timestamp_with_name(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which returns the timestamp in seconds for the last raw sample\n  on the given lookbehind window <code>d</code> per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "tfirst_over_time",
    description: `<code>tfirst_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which returns the timestamp in seconds for the first raw sample\n  on the given lookbehind window <code>d</code> per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "tlast_change_over_time",
    description: `<code>tlast_change_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which returns the timestamp in seconds for the last change\n  per each time series returned from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a> on the given lookbehind window <code>d</code>.`,
    type: "Rollup function"
  },
  {
    value: "tlast_over_time",
    description: `<code>tlast_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which is an alias for <a class="vm-link vm-link_colored" target="_blank" href="#timestamp">timestamp</a>.`,
    type: "Rollup function"
  },
  {
    value: "tmax_over_time",
    description: `<code>tmax_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which returns the timestamp in seconds for the raw sample\n  with the maximum value on the given lookbehind window <code>d</code>. It is calculated independently per each time series returned\n  from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "tmin_over_time",
    description: `<code>tmin_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which returns the timestamp in seconds for the raw sample\n  with the minimum value on the given lookbehind window <code>d</code>. It is calculated independently per each time series returned\n  from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "zscore_over_time",
    description: `<code>zscore_over_time(series_selector[d])</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup-functions">rollup function</a>, which returns <a class="vm-link vm-link_colored" target="_blank" href="https://en.wikipedia.org/wiki/Standard_score">z-score</a>\n  for raw samples on the given lookbehind window <code>d</code>. It is calculated independently per each time series returned\n  from the given <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#filtering">series_selector</a>.`,
    type: "Rollup function"
  },
  {
    value: "abs",
    description: `<code>abs(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates the absolute value for every point of every time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "absent",
    description: `<code>absent(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns 1 if <code>q</code> has no points. Otherwise, returns an empty result.`,
    type: "Transform function"
  },
  {
    value: "acos",
    description: `<code>acos(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns <a class="vm-link vm-link_colored" target="_blank" href="https://en.wikipedia.org/wiki/Inverse_trigonometric_functions">inverse cosine</a>\n  for every point of every time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "acosh",
    description: `<code>acosh(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns\n  <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}https://en.wikipedia.org/wiki/Inverse_hyperbolic_functions#Inverse_hyperbolic_cosine">inverse hyperbolic cosine</a> for every point of every time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "asin",
    description: `<code>asin(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns <a class="vm-link vm-link_colored" target="_blank" href="https://en.wikipedia.org/wiki/Inverse_trigonometric_functions">inverse sine</a>\n  for every point of every time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "asinh",
    description: `<code>asinh(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns\n  <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}https://en.wikipedia.org/wiki/Inverse_hyperbolic_functions#Inverse_hyperbolic_sine">inverse hyperbolic sine</a> for every point of every time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "atan",
    description: `<code>atan(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns <a class="vm-link vm-link_colored" target="_blank" href="https://en.wikipedia.org/wiki/Inverse_trigonometric_functions">inverse tangent</a>\n  for every point of every time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "atanh",
    description: `<code>atanh(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns\n  <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}https://en.wikipedia.org/wiki/Inverse_hyperbolic_functions#Inverse_hyperbolic_tangent">inverse hyperbolic tangent</a> for every point of every time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "bitmap_and",
    description: `<code>bitmap_and(q, mask)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates bitwise <code>v &amp; mask</code> for every <code>v</code> point of every time series returned from <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "bitmap_or",
    description: `<code>bitmap_or(q, mask)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates bitwise <code>v | mask</code> for every <code>v</code> point of every time series returned from <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "bitmap_xor",
    description: `<code>bitmap_xor(q, mask)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates bitwise <code>v ^ mask</code> for every <code>v</code> point of every time series returned from <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "buckets_limit",
    description: `<code>buckets_limit(limit, buckets)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which limits the number\n  of <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}https://valyala.medium.com/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350">histogram buckets</a> to the given <code>limit</code>.`,
    type: "Transform function"
  },
  {
    value: "ceil",
    description: `<code>ceil(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which rounds every point for every time series returned by <code>q</code> to the upper nearest integer.`,
    type: "Transform function"
  },
  {
    value: "clamp",
    description: `<code>clamp(q, min, max)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which clamps every point for every time series returned by <code>q</code> with the given <code>min</code> and <code>max</code> values.`,
    type: "Transform function"
  },
  {
    value: "clamp_max",
    description: `<code>clamp_max(q, max)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which clamps every point for every time series returned by <code>q</code> with the given <code>max</code> value.`,
    type: "Transform function"
  },
  {
    value: "clamp_min",
    description: `<code>clamp_min(q, min)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which clamps every point for every time series returned by <code>q</code> with the given <code>min</code> value.`,
    type: "Transform function"
  },
  {
    value: "cos",
    description: `<code>cos(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns <code>cos(v)</code> for every <code>v</code> point of every time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "cosh",
    description: `<code>cosh(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns <a class="vm-link vm-link_colored" target="_blank" href="https://en.wikipedia.org/wiki/Hyperbolic_functions">hyperbolic cosine</a>\n  for every point of every time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "day_of_month",
    description: `<code>day_of_month(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns the day of month for every point of every time series returned by <code>q</code>.\n  It is expected that <code>q</code> returns unix timestamps. The returned values are in the range <code>[1...31]</code>.`,
    type: "Transform function"
  },
  {
    value: "day_of_week",
    description: `<code>day_of_week(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns the day of week for every point of every time series returned by <code>q</code>.\n  It is expected that <code>q</code> returns unix timestamps. The returned values are in the range <code>[0...6]</code>, where <code>0</code> means Sunday and <code>6</code> means Saturday.`,
    type: "Transform function"
  },
  {
    value: "days_in_month",
    description: `<code>days_in_month(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns the number of days in the month identified\n  by every point of every time series returned by <code>q</code>. It is expected that <code>q</code> returns unix timestamps.\n  The returned values are in the range <code>[28...31]</code>.`,
    type: "Transform function"
  },
  {
    value: "deg",
    description: `<code>deg(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which converts <a class="vm-link vm-link_colored" target="_blank" href="https://en.wikipedia.org/wiki/Radian#Conversions">Radians to degrees</a>\n  for every point of every time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "end",
    description: `<code>end()</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns the unix timestamp in seconds for the last point.\n  It is known as <code>end</code> query arg passed to <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#range-query">/api/v1/query_range</a>.`,
    type: "Transform function"
  },
  {
    value: "exp",
    description: `<code>exp(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates the <code>e^v</code> for every point <code>v</code> of every time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "floor",
    description: `<code>floor(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which rounds every point for every time series returned by <code>q</code> to the lower nearest integer.`,
    type: "Transform function"
  },
  {
    value: "histogram_avg",
    description: `<code>histogram_avg(buckets)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates the average value for the given <code>buckets</code>.\n  It can be used for calculating the average over the given time range across multiple time series.\n  For example, <code>histogram_avg(sum(histogram_over_time(response_time_duration_seconds[5m])) by (vmrange,job))</code> would return the average response time\n  per each <code>job</code> over the last 5 minutes.`,
    type: "Transform function"
  },
  {
    value: "histogram_quantile",
    description: `<code>histogram_quantile(phi, buckets)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates <code>phi</code>-<a class="vm-link vm-link_colored" target="_blank" href="https://en.wikipedia.org/wiki/Percentile">percentile</a>\n  over the given <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}https://valyala.medium.com/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350">histogram buckets</a>.\n  <code>phi</code> must be in the range <code>[0...1]</code>. For example, <code>histogram_quantile(0.5, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))</code>\n  would return median request duration for all the requests during the last 5 minutes.`,
    type: "Transform function"
  },
  {
    value: "histogram_quantiles",
    description: `<code>histogram_quantiles("phiLabel", phi1, ..., phiN, buckets)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates the given <code>phi*</code>-quantiles\n  over the given <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}https://valyala.medium.com/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350">histogram buckets</a>.\n  Argument <code>phi*</code> must be in the range <code>[0...1]</code>. For example, <code>histogram_quantiles('le', 0.3, 0.5, sum(rate(http_request_duration_seconds_bucket[5m]) by (le))</code>.\n  Each calculated quantile is returned in a separate time series with the corresponding <code>{phiLabel="phi*"}</code> label.`,
    type: "Transform function"
  },
  {
    value: "histogram_share",
    description: `<code>histogram_share(le, buckets)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates the share (in the range <code>[0...1]</code>)\n  for <code>buckets</code> that fall below <code>le</code>. This function is useful for calculating SLI and SLO. This is inverse to <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#histogram_quantile">histogram_quantile</a>.`,
    type: "Transform function"
  },
  {
    value: "histogram_stddev",
    description: `<code>histogram_stddev(buckets)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates standard deviation for the given <code>buckets</code>.`,
    type: "Transform function"
  },
  {
    value: "histogram_stdvar",
    description: `<code>histogram_stdvar(buckets)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates standard variance for the given <code>buckets</code>.\n  It can be used for calculating standard deviation over the given time range across multiple time series.\n  For example, <code>histogram_stdvar(sum(histogram_over_time(temperature[24])) by (vmrange,country))</code> would return standard deviation\n  for the temperature per each country over the last 24 hours.`,
    type: "Transform function"
  },
  {
    value: "hour",
    description: `<code>hour(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns the hour for every point of every time series returned by <code>q</code>.\n  It is expected that <code>q</code> returns unix timestamps. The returned values are in the range <code>[0...23]</code>.`,
    type: "Transform function"
  },
  {
    value: "interpolate",
    description: `<code>interpolate(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which fills gaps with linearly interpolated values calculated\n  from the last and the next non-empty points per each time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "keep_last_value",
    description: `<code>keep_last_value(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which fills gaps with the value of the last non-empty point\n  in every time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "keep_next_value",
    description: `<code>keep_next_value(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which fills gaps with the value of the next non-empty point\n  in every time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "limit_offset",
    description: `<code>limit_offset(limit, offset, q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which skips <code>offset</code> time series from series returned by <code>q</code>\n  and then returns up to <code>limit</code> of the remaining time series per each group.`,
    type: "Transform function"
  },
  {
    value: "ln",
    description: `<code>ln(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates <code>ln(v)</code> for every point <code>v</code> of every time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "log2",
    description: `<code>log2(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates <code>log2(v)</code> for every point <code>v</code> of every time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "log10",
    description: `<code>log10(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates <code>log10(v)</code> for every point <code>v</code> of every time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "minute",
    description: `<code>minute(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns the minute for every point of every time series returned by <code>q</code>.\n  It is expected that <code>q</code> returns unix timestamps. The returned values are in the range <code>[0...59]</code>.`,
    type: "Transform function"
  },
  {
    value: "month",
    description: `<code>month(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns the month for every point of every time series returned by <code>q</code>.\n  It is expected that <code>q</code> returns unix timestamps. The returned values are in the range <code>[1...12]</code>, where <code>1</code> means January and <code>12</code> means December.`,
    type: "Transform function"
  },
  {
    value: "now",
    description: `<code>now()</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns the current timestamp as a floating-point value in seconds.`,
    type: "Transform function"
  },
  {
    value: "pi",
    description: `<code>pi()</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns <a class="vm-link vm-link_colored" target="_blank" href="https://en.wikipedia.org/wiki/Pi">Pi number</a>.`,
    type: "Transform function"
  },
  {
    value: "rad",
    description: `<code>rad(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which converts <a class="vm-link vm-link_colored" target="_blank" href="https://en.wikipedia.org/wiki/Radian#Conversions">degrees to Radians</a>\n  for every point of every time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "prometheus_buckets",
    description: `<code>prometheus_buckets(buckets)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which converts\n  <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}https://valyala.medium.com/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350">VictoriaMetrics histogram buckets</a> with <code>vmrange</code> labels\n  to Prometheus histogram buckets with <code>le</code> labels. This may be useful for building heatmaps in Grafana.`,
    type: "Transform function"
  },
  {
    value: "rand",
    description: `<code>rand(seed)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns pseudo-random numbers on the range <code>[0...1]</code> with even distribution.\n  Optional <code>seed</code> can be used as a seed for pseudo-random number generator.`,
    type: "Transform function"
  },
  {
    value: "rand_exponential",
    description: `<code>rand_exponential(seed)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns pseudo-random numbers\n  with <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}https://en.wikipedia.org/wiki/Exponential_distribution">exponential distribution</a>. Optional <code>seed</code> can be used as a seed for pseudo-random number generator.`,
    type: "Transform function"
  },
  {
    value: "rand_normal",
    description: `<code>rand_normal(seed)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns pseudo-random numbers\n  with <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}https://en.wikipedia.org/wiki/Normal_distribution">normal distribution</a>. Optional <code>seed</code> can be used as a seed for pseudo-random number generator.`,
    type: "Transform function"
  },
  {
    value: "range_avg",
    description: `<code>range_avg(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates the avg value across points per each time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "range_first",
    description: `<code>range_first(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns the value for the first point per each time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "range_last",
    description: `<code>range_last(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns the value for the last point per each time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "range_linear_regression",
    description: `<code>range_linear_regression(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates <a class="vm-link vm-link_colored" target="_blank" href="https://en.wikipedia.org/wiki/Simple_linear_regression">simple linear regression</a>\n  over the selected time range per each time series returned by <code>q</code>. This function is useful for capacity planning and predictions.`,
    type: "Transform function"
  },
  {
    value: "range_mad",
    description: `<code>range_mad(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates the <a class="vm-link vm-link_colored" target="_blank" href="https://en.wikipedia.org/wiki/Median_absolute_deviation">median absolute deviation</a>\n  across points per each time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "range_max",
    description: `<code>range_max(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates the max value across points per each time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "range_median",
    description: `<code>range_median(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates the median value across points per each time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "range_min",
    description: `<code>range_min(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates the min value across points per each time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "range_normalize",
    description: `<code>range_normalize(q1, ...)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which normalizes values for time series returned by <code>q1, ...</code> into <code>[0 ... 1]</code> range.\n  This function is useful for correlating time series with distinct value ranges.`,
    type: "Transform function"
  },
  {
    value: "range_quantile",
    description: `<code>range_quantile(phi, q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns <code>phi</code>-quantile across points per each time series returned by <code>q</code>.\n  <code>phi</code> must be in the range <code>[0...1]</code>.`,
    type: "Transform function"
  },
  {
    value: "range_stddev",
    description: `<code>range_stddev(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates <a class="vm-link vm-link_colored" target="_blank" href="https://en.wikipedia.org/wiki/Standard_deviation">standard deviation</a>\n  per each time series returned by <code>q</code> on the selected time range.`,
    type: "Transform function"
  },
  {
    value: "range_stdvar",
    description: `<code>range_stdvar(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates <a class="vm-link vm-link_colored" target="_blank" href="https://en.wikipedia.org/wiki/Variance">standard variance</a>\n  per each time series returned by <code>q</code> on the selected time range.`,
    type: "Transform function"
  },
  {
    value: "range_sum",
    description: `<code>range_sum(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates the sum of points per each time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "range_trim_outliers",
    description: `<code>range_trim_outliers(k, q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which drops points located farther than <code>k*range_mad(q)</code>\n  from the <code>range_median(q)</code>. E.g. it is equivalent to the following query: <code>q ifnot (abs(q - range_median(q)) &gt; k*range_mad(q))</code>.`,
    type: "Transform function"
  },
  {
    value: "range_trim_spikes",
    description: `<code>range_trim_spikes(phi, q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which drops <code>phi</code> percent of biggest spikes from time series returned by <code>q</code>.\n  The <code>phi</code> must be in the range <code>[0..1]</code>, where <code>0</code> means <code>0%</code> and <code>1</code> means <code>100%</code>.`,
    type: "Transform function"
  },
  {
    value: "range_trim_zscore",
    description: `<code>range_trim_zscore(z, q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which drops points located farther than <code>z*range_stddev(q)</code>\n  from the <code>range_avg(q)</code>. E.g. it is equivalent to the following query: <code>q ifnot (abs(q - range_avg(q)) &gt; z*range_avg(q))</code>.`,
    type: "Transform function"
  },
  {
    value: "range_zscore",
    description: `<code>range_zscore(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates <a class="vm-link vm-link_colored" target="_blank" href="https://en.wikipedia.org/wiki/Standard_score">z-score</a>\n  for points returned by <code>q</code>, e.g. it is equivalent to the following query: <code>(q - range_avg(q)) / range_stddev(q)</code>.`,
    type: "Transform function"
  },
  {
    value: "remove_resets",
    description: `<code>remove_resets(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which removes counter resets from time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "round",
    description: `<code>round(q, nearest)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which rounds every point of every time series returned by <code>q</code> to the <code>nearest</code> multiple.\n  If <code>nearest</code> is missing then the rounding is performed to the nearest integer.`,
    type: "Transform function"
  },
  {
    value: "ru",
    description: `<code>ru(free, max)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates resource utilization in the range <code>[0%...100%]</code> for the given <code>free</code> and <code>max</code> resources.\n  For instance, <code>ru(node_memory_MemFree_bytes, node_memory_MemTotal_bytes)</code> returns memory utilization over <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}https://github.com/prometheus/node_exporter">node_exporter</a> metrics.`,
    type: "Transform function"
  },
  {
    value: "running_avg",
    description: `<code>running_avg(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates the running avg per each time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "running_max",
    description: `<code>running_max(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates the running max per each time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "running_min",
    description: `<code>running_min(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates the running min per each time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "running_sum",
    description: `<code>running_sum(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates the running sum per each time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "scalar",
    description: `<code>scalar(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns <code>q</code> if <code>q</code> contains only a single time series. Otherwise, it returns nothing.`,
    type: "Transform function"
  },
  {
    value: "sgn",
    description: `<code>sgn(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns <code>1</code> if <code>v&gt;0</code>, <code>-1</code> if <code>v&lt;0</code> and <code>0</code> if <code>v==0</code> for every point <code>v</code>\n  of every time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "sin",
    description: `<code>sin(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns <code>sin(v)</code> for every <code>v</code> point of every time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "sinh",
    description: `<code>sinh(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns <a class="vm-link vm-link_colored" target="_blank" href="https://en.wikipedia.org/wiki/Hyperbolic_functions">hyperbolic sine</a>\n  for every point of every time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "tan",
    description: `<code>tan(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns <code>tan(v)</code> for every <code>v</code> point of every time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "tanh",
    description: `<code>tanh(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns <a class="vm-link vm-link_colored" target="_blank" href="https://en.wikipedia.org/wiki/Hyperbolic_functions">hyperbolic tangent</a>\n  for every point of every time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "smooth_exponential",
    description: `<code>smooth_exponential(q, sf)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which smooths points per each time series returned\n  by <code>q</code> using <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}https://en.wikipedia.org/wiki/Moving_average#Exponential_moving_average">exponential moving average</a> with the given smooth factor <code>sf</code>.`,
    type: "Transform function"
  },
  {
    value: "sort",
    description: `<code>sort(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which sorts series in ascending order by the last point in every time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "sort_desc",
    description: `<code>sort_desc(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which sorts series in descending order by the last point in every time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "sqrt",
    description: `<code>sqrt(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which calculates square root for every point of every time series returned by <code>q</code>.`,
    type: "Transform function"
  },
  {
    value: "start",
    description: `<code>start()</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns unix timestamp in seconds for the first point.`,
    type: "Transform function"
  },
  {
    value: "step",
    description: `<code>step()</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns the step in seconds (aka interval) between the returned points.\n  It is known as <code>step</code> query arg passed to <a class="vm-link vm-link_colored" target="_blank" href="https://docs.victoriametrics.com/keyConcepts.html#range-query">/api/v1/query_range</a>.`,
    type: "Transform function"
  },
  {
    value: "time",
    description: `<code>time()</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns unix timestamp for every returned point.`,
    type: "Transform function"
  },
  {
    value: "timezone_offset",
    description: `<code>timezone_offset(tz)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns offset in seconds for the given timezone <code>tz</code> relative to UTC.\n  This can be useful when combining with datetime-related functions. For example, <code>day_of_week(time()+timezone_offset("America/Los_Angeles"))</code>\n  would return weekdays for <code>America/Los_Angeles</code> time zone.`,
    type: "Transform function"
  },
  {
    value: "ttf",
    description: `<code>ttf(free)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which estimates the time in seconds needed to exhaust <code>free</code> resources.\n  For instance, <code>ttf(node_filesystem_avail_byte)</code> returns the time to storage space exhaustion. This function may be useful for capacity planning.`,
    type: "Transform function"
  },
  {
    value: "union",
    description: `<code>union(q1, ..., qN)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns a union of time series returned from <code>q1</code>, ..., <code>qN</code>.\n  The <code>union</code> function name can be skipped - the following queries are equivalent: <code>union(q1, q2)</code> and <code>(q1, q2)</code>.`,
    type: "Transform function"
  },
  {
    value: "vector",
    description: `<code>vector(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns <code>q</code>, e.g. it does nothing in MetricsQL.`,
    type: "Transform function"
  },
  {
    value: "year",
    description: `<code>year(q)</code> is a <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#transform-functions">transform function</a>, which returns the year for every point of every time series returned by <code>q</code>.\n  It is expected that <code>q</code> returns unix timestamps.`,
    type: "Transform function"
  },
  {
    value: "alias",
    description: `<code>alias(q, "name")</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#label-manipulation-functions">label manipulation function</a>, which sets the given <code>name</code> to all the time series returned by <code>q</code>.\n  For example, <code>alias(up, "foobar")</code> would rename <code>up</code> series to <code>foobar</code> series.`,
    type: "Label manipulation function"
  },
  {
    value: "drop_common_labels",
    description: `<code>drop_common_labels(q1, ...., qN)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#label-manipulation-functions">label manipulation function</a>, which drops common <code>label="value"</code> pairs\n  among time series returned from <code>q1, ..., qN</code>.`,
    type: "Label manipulation function"
  },
  {
    value: "label_copy",
    description: `<code>label_copy(q, "src_label1", "dst_label1", ..., "src_labelN", "dst_labelN")</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#label-manipulation-functions">label manipulation function</a>,\n  which copies label values from <code>src_label*</code> to <code>dst_label*</code> for all the time series returned by <code>q</code>.\n  If <code>src_label</code> is empty, then the corresponding <code>dst_label</code> is left untouched.`,
    type: "Label manipulation function"
  },
  {
    value: "label_del",
    description: `<code>label_del(q, "label1", ..., "labelN")</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#label-manipulation-functions">label manipulation function</a>, which deletes the given <code>label*</code> labels\n  from all the time series returned by <code>q</code>.`,
    type: "Label manipulation function"
  },
  {
    value: "label_graphite_group",
    description: `<code>label_graphite_group(q, groupNum1, ... groupNumN)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#label-manipulation-functions">label manipulation function</a>, which replaces metric names\n  returned from <code>q</code> with the given Graphite group values concatenated via <code>.</code> char.`,
    type: "Label manipulation function"
  },
  {
    value: "label_join",
    description: `<code>label_join(q, "dst_label", "separator", "src_label1", ..., "src_labelN")</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#label-manipulation-functions">label manipulation function</a>,\n  which joins <code>src_label*</code> values with the given <code>separator</code> and stores the result in <code>dst_label</code>.\n  This is performed individually per each time series returned by <code>q</code>.\n  For example, <code>label_join(up{instance="xxx",job="yyy"}, "foo", "-", "instance", "job")</code> would store <code>xxx-yyy</code> label value into <code>foo</code> label.`,
    type: "Label manipulation function"
  },
  {
    value: "label_keep",
    description: `<code>label_keep(q, "label1", ..., "labelN")</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#label-manipulation-functions">label manipulation function</a>, which deletes all the labels\n  except of the listed <code>label*</code> labels in all the time series returned by <code>q</code>.`,
    type: "Label manipulation function"
  },
  {
    value: "label_lowercase",
    description: `<code>label_lowercase(q, "label1", ..., "labelN")</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#label-manipulation-functions">label manipulation function</a>, which lowercases values\n  for the given <code>label*</code> labels in all the time series returned by <code>q</code>.`,
    type: "Label manipulation function"
  },
  {
    value: "label_map",
    description: `<code>label_map(q, "label", "src_value1", "dst_value1", ..., "src_valueN", "dst_valueN")</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#label-manipulation-functions">label manipulation function</a>,\n  which maps <code>label</code> values from <code>src_*</code> to <code>dst*</code> for all the time series returned by <code>q</code>.`,
    type: "Label manipulation function"
  },
  {
    value: "label_match",
    description: `<code>label_match(q, "label", "regexp")</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#label-manipulation-functions">label manipulation function</a>,\n  which drops time series from <code>q</code> with <code>label</code> not matching the given <code>regexp</code>.\n  This function can be useful after <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup">rollup</a>-like functions, which may return multiple time series for every input series.`,
    type: "Label manipulation function"
  },
  {
    value: "label_mismatch",
    description: `<code>label_mismatch(q, "label", "regexp")</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#label-manipulation-functions">label manipulation function</a>,\n  which drops time series from <code>q</code> with <code>label</code> matching the given <code>regexp</code>.\n  This function can be useful after <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#rollup">rollup</a>-like functions, which may return multiple time series for every input series.`,
    type: "Label manipulation function"
  },
  {
    value: "label_move",
    description: `<code>label_move(q, "src_label1", "dst_label1", ..., "src_labelN", "dst_labelN")</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#label-manipulation-functions">label manipulation function</a>,\n  which moves label values from <code>src_label*</code> to <code>dst_label*</code> for all the time series returned by <code>q</code>.\n  If <code>src_label</code> is empty, then the corresponding <code>dst_label</code> is left untouched.`,
    type: "Label manipulation function"
  },
  {
    value: "label_replace",
    description: `<code>label_replace(q, "dst_label", "replacement", "src_label", "regex")</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#label-manipulation-functions">label manipulation function</a>,\n  which applies the given <code>regex</code> to <code>src_label</code> and stores the <code>replacement</code> in <code>dst_label</code> if the given <code>regex</code> matches <code>src_label</code>.\n  The <code>replacement</code> may contain references to regex captures such as <code>$1</code>, <code>$2</code>, etc.\n  These references are substituted by the corresponding regex captures.\n  For example, <code>label_replace(up{job="node-exporter"}, "foo", "bar-$1", "job", "node-(.+)")</code> would store <code>bar-exporter</code> label value into <code>foo</code> label.`,
    type: "Label manipulation function"
  },
  {
    value: "label_set",
    description: `<code>label_set(q, "label1", "value1", ..., "labelN", "valueN")</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#label-manipulation-functions">label manipulation function</a>,\n  which sets <code>{label1="value1", ..., labelN="valueN"}</code> labels to all the time series returned by <code>q</code>.`,
    type: "Label manipulation function"
  },
  {
    value: "label_transform",
    description: `<code>label_transform(q, "label", "regexp", "replacement")</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#label-manipulation-functions">label manipulation function</a>,\n  which substitutes all the <code>regexp</code> occurrences by the given <code>replacement</code> in the given <code>label</code>.`,
    type: "Label manipulation function"
  },
  {
    value: "label_uppercase",
    description: `<code>label_uppercase(q, "label1", ..., "labelN")</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#label-manipulation-functions">label manipulation function</a>,\n  which uppercases values for the given <code>label*</code> labels in all the time series returned by <code>q</code>.`,
    type: "Label manipulation function"
  },
  {
    value: "label_value",
    description: `<code>label_value(q, "label")</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#label-manipulation-functions">label manipulation function</a>, which returns numeric values\n  for the given <code>label</code> for every time series returned by <code>q</code>.`,
    type: "Label manipulation function"
  },
  {
    value: "sort_by_label",
    description: `<code>sort_by_label(q, label1, ... labelN)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#label-manipulation-functions">label manipulation function</a>, which sorts series in ascending order by the given set of labels.\n  For example, <code>sort_by_label(foo, "bar")</code> would sort <code>foo</code> series by values of the label <code>bar</code> in these series.`,
    type: "Label manipulation function"
  },
  {
    value: "sort_by_label_desc",
    description: `<code>sort_by_label_desc(q, label1, ... labelN)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#label-manipulation-functions">label manipulation function</a>, which sorts series in descending order by the given set of labels.\n  For example, <code>sort_by_label(foo, "bar")</code> would sort <code>foo</code> series by values of the label <code>bar</code> in these series.`,
    type: "Label manipulation function"
  },
  {
    value: "sort_by_label_numeric",
    description: `<code>sort_by_label_numeric(q, label1, ... labelN)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#label-manipulation-functions">label manipulation function</a>, which sorts series in ascending order by the given set of labels\n  using <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}https://www.gnu.org/software/coreutils/manual/html_node/Version-sort-is-not-the-same-as-numeric-sort.html">numeric sort</a>.\n  For example, if <code>foo</code> series have <code>bar</code> label with values <code>1</code>, <code>101</code>, <code>15</code> and <code>2</code>, then <code>sort_by_label_numeric(foo, "bar")</code> would return series\n  in the following order of <code>bar</code> label values: <code>1</code>, <code>2</code>, <code>15</code> and <code>101</code>.`,
    type: "Label manipulation function"
  },
  {
    value: "sort_by_label_numeric_desc",
    description: `<code>sort_by_label_numeric_desc(q, label1, ... labelN)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#label-manipulation-functions">label manipulation function</a>, which sorts series in descending order\n  by the given set of labels using <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}https://www.gnu.org/software/coreutils/manual/html_node/Version-sort-is-not-the-same-as-numeric-sort.html">numeric sort</a>.\n  For example, if <code>foo</code> series have <code>bar</code> label with values <code>1</code>, <code>101</code>, <code>15</code> and <code>2</code>, then <code>sort_by_label_numeric(foo, "bar")</code>\n  would return series in the following order of <code>bar</code> label values: <code>101</code>, <code>15</code>, <code>2</code> and <code>1</code>.`,
    type: "Label manipulation function"
  },
  {
    value: "any",
    description: `<code>any(q) by (group_labels)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns a single series per <code>group_labels</code> out of time series returned by <code>q</code>.`,
    type: "Aggregate function"
  },
  {
    value: "avg",
    description: `<code>avg(q) by (group_labels)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns the average value per <code>group_labels</code> for time series returned by <code>q</code>.\n  The aggregate is calculated individually per each group of points with the same timestamp.`,
    type: "Aggregate function"
  },
  {
    value: "bottomk",
    description: `<code>bottomk(k, q)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns up to <code>k</code> points with the smallest values across all the time series returned by <code>q</code>.\n  The aggregate is calculated individually per each group of points with the same timestamp.`,
    type: "Aggregate function"
  },
  {
    value: "bottomk_avg",
    description: `<code>bottomk_avg(k, q, "other_label=other_value")</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns up to <code>k</code> time series from <code>q</code> with the smallest averages.\n  If an optional <code>other_label=other_value</code> arg is set, then the sum of the remaining time series is returned with the given label.\n  For example, <code>bottomk_avg(3, sum(process_resident_memory_bytes) by (job), "job=other")</code> would return up to 3 time series\n  with the smallest averages plus a time series with <code>{job="other"}</code> label with the sum of the remaining series if any.`,
    type: "Aggregate function"
  },
  {
    value: "bottomk_last",
    description: `<code>bottomk_last(k, q, "other_label=other_value")</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns up to <code>k</code> time series from <code>q</code> with the smallest last values.\n  If an optional <code>other_label=other_value</code> arg is set, then the sum of the remaining time series is returned with the given label.\n  For example, <code>bottomk_max(3, sum(process_resident_memory_bytes) by (job), "job=other")</code> would return up to 3 time series\n  with the smallest maximums plus a time series with <code>{job="other"}</code> label with the sum of the remaining series if any.`,
    type: "Aggregate function"
  },
  {
    value: "bottomk_max",
    description: `<code>bottomk_max(k, q, "other_label=other_value")</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns up to <code>k</code> time series from <code>q</code> with the smallest maximums.\n  If an optional <code>other_label=other_value</code> arg is set, then the sum of the remaining time series is returned with the given label.\n  For example, <code>bottomk_max(3, sum(process_resident_memory_bytes) by (job), "job=other")</code> would return up to 3 time series\n  with the smallest maximums plus a time series with <code>{job="other"}</code> label with the sum of the remaining series if any.`,
    type: "Aggregate function"
  },
  {
    value: "bottomk_median",
    description: `<code>bottomk_median(k, q, "other_label=other_value")</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns up to <code>k</code> time series from <code>q</code> with the smallest medians.\n  If an optional<code>other_label=other_value</code> arg is set, then the sum of the remaining time series is returned with the given label.\n  For example, <code>bottomk_median(3, sum(process_resident_memory_bytes) by (job), "job=other")</code> would return up to 3 time series\n  with the smallest medians plus a time series with <code>{job="other"}</code> label with the sum of the remaining series if any.`,
    type: "Aggregate function"
  },
  {
    value: "bottomk_min",
    description: `<code>bottomk_min(k, q, "other_label=other_value")</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns up to <code>k</code> time series from <code>q</code> with the smallest minimums.\n  If an optional <code>other_label=other_value</code> arg is set, then the sum of the remaining time series is returned with the given label.\n  For example, <code>bottomk_min(3, sum(process_resident_memory_bytes) by (job), "job=other")</code> would return up to 3 time series\n  with the smallest minimums plus a time series with <code>{job="other"}</code> label with the sum of the remaining series if any.`,
    type: "Aggregate function"
  },
  {
    value: "count",
    description: `<code>count(q) by (group_labels)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns the number of non-empty points per <code>group_labels</code>\n  for time series returned by <code>q</code>. The aggregate is calculated individually per each group of points with the same timestamp.`,
    type: "Aggregate function"
  },
  {
    value: "count_values",
    description: `<code>count_values("label", q)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which counts the number of points with the same value\n  and stores the counts in a time series with an additional <code>label</code>, which contains each initial value.\n  The aggregate is calculated individually per each group of points with the same timestamp.`,
    type: "Aggregate function"
  },
  {
    value: "distinct",
    description: `<code>distinct(q)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which calculates the number of unique values per each group of points with the same timestamp.`,
    type: "Aggregate function"
  },
  {
    value: "geomean",
    description: `<code>geomean(q)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which calculates geometric mean per each group of points with the same timestamp.`,
    type: "Aggregate function"
  },
  {
    value: "group",
    description: `<code>group(q) by (group_labels)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns <code>1</code> per each <code>group_labels</code> for time series returned by <code>q</code>.`,
    type: "Aggregate function"
  },
  {
    value: "histogram",
    description: `<code>histogram(q)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which calculates\n  <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}https://valyala.medium.com/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350">VictoriaMetrics histogram</a>\n  per each group of points with the same timestamp. Useful for visualizing big number of time series via a heatmap.\n  See <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}https://medium.com/@valyala/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350">this article</a> for more details.`,
    type: "Aggregate function"
  },
  {
    value: "limitk",
    description: `<code>limitk(k, q) by (group_labels)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns up to <code>k</code> time series per each <code>group_labels</code>\n  out of time series returned by <code>q</code>. The returned set of time series remain the same across calls.`,
    type: "Aggregate function"
  },
  {
    value: "mad",
    description: `<code>mad(q) by (group_labels)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns the <a class="vm-link vm-link_colored" target="_blank" href="https://en.wikipedia.org/wiki/Median_absolute_deviation">Median absolute deviation</a>\n  per each <code>group_labels</code> for all the time series returned by <code>q</code>. The aggregate is calculated individually per each group of points with the same timestamp.`,
    type: "Aggregate function"
  },
  {
    value: "max",
    description: `<code>max(q) by (group_labels)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns the maximum value per each <code>group_labels</code>\n  for all the time series returned by <code>q</code>. The aggregate is calculated individually per each group of points with the same timestamp.`,
    type: "Aggregate function"
  },
  {
    value: "median",
    description: `<code>median(q) by (group_labels)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns the median value per each <code>group_labels</code>\n  for all the time series returned by <code>q</code>. The aggregate is calculated individually per each group of points with the same timestamp.`,
    type: "Aggregate function"
  },
  {
    value: "min",
    description: `<code>min(q) by (group_labels)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns the minimum value per each <code>group_labels</code>\n  for all the time series returned by <code>q</code>. The aggregate is calculated individually per each group of points with the same timestamp.`,
    type: "Aggregate function"
  },
  {
    value: "mode",
    description: `<code>mode(q) by (group_labels)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns <a class="vm-link vm-link_colored" target="_blank" href="https://en.wikipedia.org/wiki/Mode_(statistics">mode</a>)\n  per each <code>group_labels</code> for all the time series returned by <code>q</code>. The aggregate is calculated individually per each group of points with the same timestamp.`,
    type: "Aggregate function"
  },
  {
    value: "outliers_mad",
    description: `<code>outliers_mad(tolerance, q)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns time series from <code>q</code> with at least\n  a single point outside <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}https://en.wikipedia.org/wiki/Median_absolute_deviation">Median absolute deviation</a> (aka MAD) multiplied by <code>tolerance</code>.\n  E.g. it returns time series with at least a single point below <code>median(q) - mad(q)</code> or a single point above <code>median(q) + mad(q)</code>.`,
    type: "Aggregate function"
  },
  {
    value: "outliersk",
    description: `<code>outliersk(k, q)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns up to <code>k</code> time series with the biggest standard deviation (aka outliers)\n  out of time series returned by <code>q</code>.`,
    type: "Aggregate function"
  },
  {
    value: "quantile",
    description: `<code>quantile(phi, q) by (group_labels)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which calculates <code>phi</code>-quantile per each <code>group_labels</code>\n  for all the time series returned by <code>q</code>. <code>phi</code> must be in the range <code>[0...1]</code>.\n  The aggregate is calculated individually per each group of points with the same timestamp.`,
    type: "Aggregate function"
  },
  {
    value: "quantiles",
    description: `<code>quantiles("phiLabel", phi1, ..., phiN, q)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which calculates <code>phi*</code>-quantiles for all the time series\n  returned by <code>q</code> and return them in time series with <code>{phiLabel="phi*"}</code> label. <code>phi*</code> must be in the range <code>[0...1]</code>.\n  The aggregate is calculated individually per each group of points with the same timestamp.`,
    type: "Aggregate function"
  },
  {
    value: "share",
    description: `<code>share(q) by (group_labels)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns shares in the range <code>[0..1]</code>\n  for every non-negative points returned by <code>q</code> per each timestamp, so the sum of shares per each <code>group_labels</code> equals 1.`,
    type: "Aggregate function"
  },
  {
    value: "stddev",
    description: `<code>stddev(q) by (group_labels)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which calculates standard deviation per each <code>group_labels</code>\n  for all the time series returned by <code>q</code>. The aggregate is calculated individually per each group of points with the same timestamp.`,
    type: "Aggregate function"
  },
  {
    value: "stdvar",
    description: `<code>stdvar(q) by (group_labels)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which calculates standard variance per each <code>group_labels</code>\n  for all the time series returned by <code>q</code>. The aggregate is calculated individually per each group of points with the same timestamp.`,
    type: "Aggregate function"
  },
  {
    value: "sum",
    description: `<code>sum(q) by (group_labels)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns the sum per each <code>group_labels</code>\n  for all the time series returned by <code>q</code>. The aggregate is calculated individually per each group of points with the same timestamp.`,
    type: "Aggregate function"
  },
  {
    value: "sum2",
    description: `<code>sum2(q) by (group_labels)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which calculates the sum of squares per each <code>group_labels</code>\n  for all the time series returned by <code>q</code>. The aggregate is calculated individually per each group of points with the same timestamp.`,
    type: "Aggregate function"
  },
  {
    value: "topk",
    description: `<code>topk(k, q)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns up to <code>k</code> points with the biggest values across all the time series returned by <code>q</code>.\n  The aggregate is calculated individually per each group of points with the same timestamp.`,
    type: "Aggregate function"
  },
  {
    value: "topk_avg",
    description: `<code>topk_avg(k, q, "other_label=other_value")</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns up to <code>k</code> time series from <code>q</code> with the biggest averages.\n  If an optional <code>other_label=other_value</code> arg is set, then the sum of the remaining time series is returned with the given label.\n  For example, <code>topk_avg(3, sum(process_resident_memory_bytes) by (job), "job=other")</code> would return up to 3 time series with the biggest averages\n  plus a time series with <code>{job="other"}</code> label with the sum of the remaining series if any.`,
    type: "Aggregate function"
  },
  {
    value: "topk_last",
    description: `<code>topk_last(k, q, "other_label=other_value")</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns up to <code>k</code> time series from <code>q</code> with the biggest last values.\n  If an optional <code>other_label=other_value</code> arg is set, then the sum of the remaining time series is returned with the given label.\n  For example, <code>topk_max(3, sum(process_resident_memory_bytes) by (job), "job=other")</code> would return up to 3 time series with the biggest maximums\n  plus a time series with <code>{job="other"}</code> label with the sum of the remaining series if any.`,
    type: "Aggregate function"
  },
  {
    value: "topk_max",
    description: `<code>topk_max(k, q, "other_label=other_value")</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns up to <code>k</code> time series from <code>q</code> with the biggest maximums.\n  If an optional <code>other_label=other_value</code> arg is set, then the sum of the remaining time series is returned with the given label.\n  For example, <code>topk_max(3, sum(process_resident_memory_bytes) by (job), "job=other")</code> would return up to 3 time series with the biggest maximums\n  plus a time series with <code>{job="other"}</code> label with the sum of the remaining series if any.`,
    type: "Aggregate function"
  },
  {
    value: "topk_median",
    description: `<code>topk_median(k, q, "other_label=other_value")</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns up to <code>k</code> time series from <code>q</code> with the biggest medians.\n  If an optional <code>other_label=other_value</code> arg is set, then the sum of the remaining time series is returned with the given label.\n  For example, <code>topk_median(3, sum(process_resident_memory_bytes) by (job), "job=other")</code> would return up to 3 time series with the biggest medians\n  plus a time series with <code>{job="other"}</code> label with the sum of the remaining series if any.`,
    type: "Aggregate function"
  },
  {
    value: "topk_min",
    description: `<code>topk_min(k, q, "other_label=other_value")</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns up to <code>k</code> time series from <code>q</code> with the biggest minimums.\n  If an optional <code>other_label=other_value</code> arg is set, then the sum of the remaining time series is returned with the given label.\n  For example, <code>topk_min(3, sum(process_resident_memory_bytes) by (job), "job=other")</code> would return up to 3 time series with the biggest minimums\n  plus a time series with <code>{job="other"}</code> label with the sum of the remaining series if any.`,
    type: "Aggregate function"
  },
  {
    value: "zscore",
    description: `<code>zscore(q) by (group_labels)</code> is <a class="vm-link vm-link_colored" target="_blank" href="${docsUrl}#aggregate-functions">aggregate function</a>, which returns <a class="vm-link vm-link_colored" target="_blank" href="https://en.wikipedia.org/wiki/Standard_score">z-score</a> values\n  per each <code>group_labels</code> for all the time series returned by <code>q</code>. The aggregate is calculated individually per each group of points with the same timestamp.\n  This function is useful for detecting anomalies in the group of related time series.`,
    type: "Aggregate function"
  }
].map(fn => ({ ...fn, icon: <FunctionIcon /> }));
