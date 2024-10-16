import React, { FC, useEffect } from "preact/compat";
import "./style.scss";
import TextField from "../../components/Main/TextField/TextField";
import { useCallback, useState } from "react";
import Button from "../../components/Main/Button/Button";
import { PlayIcon, WikiIcon } from "../../components/Main/Icons";
import { useDebugRetentionFilters } from "./hooks/useDebugRetentionFilters";
import Spinner from "../../components/Main/Spinner/Spinner";
import { useSearchParams } from "react-router-dom";

const example = {
  flags: `-retentionPeriod=1y
-retentionFilters={env!="prod"}:2w
`,
  metrics: `up
up{env="dev"}
up{env="prod"}`,
};

const RetentionFilters: FC = () => {
  const [searchParams] = useSearchParams();

  const { data, loading, error, metricsError, flagsError, applyFilters } = useDebugRetentionFilters();
  const [metrics, setMetrics] = useState(searchParams.get("metrics") || "");
  const [flags, setFlags] = useState(searchParams.get("flags") || "");

  const handleMetricsChangeInput = useCallback((val: string) => {
    setMetrics(val);
  }, [setMetrics]);

  const handleFlagsChangeInput = useCallback((val: string) => {
    setFlags(val);
  }, [setFlags]);

  const handleApplyFilters = useCallback(() => {
    applyFilters(flags, metrics);
  }, [applyFilters, flags, metrics]);

  const handleRunExample = useCallback(() => {
    const { flags, metrics } = example;
    setFlags(flags);
    setMetrics(metrics);
    applyFilters(flags, metrics);
    searchParams.set("flags", flags);
    searchParams.set("metrics", metrics);
  }, [example, setFlags, setMetrics, searchParams]);

  useEffect(() => {
    if (flags && metrics) handleApplyFilters();
  }, []);

  const rows = [];
  for (const [key, value] of data) {
    rows.push(<tr className="vm-table__row">
      <td className="vm-table-cell">{key}</td>
      <td className="vm-table-cell">{value}</td>
    </tr>);
  }
  return (
    <section className="vm-retention-filters">
      {loading && <Spinner/>}

      <div className="vm-retention-filters-body vm-block">
        <div className="vm-retention-filters-body__expr">
          <div className="vm-retention-filters-body__title">
            <p>Provide a list of flags for retention configuration. Note that
              only <code>-retentionPeriod</code> and <code>-retentionFilters</code> flags are
              supported.</p>
          </div>
          <TextField
            type="textarea"
            label="Flags"
            value={flags}
            error={error || flagsError}
            autofocus
            onEnter={handleApplyFilters}
            onChange={handleFlagsChangeInput}
            placeholder={"-retentionPeriod=4w -retentionFilters=up{env=\"dev\"}:2w"}
          />
        </div>
        <div className="vm-retention-filters-body__expr">
          <div className="vm-retention-filters-body__title">
            <p>Provide a list of metrics to check retention configuration.</p>
          </div>
          <TextField
            type="textarea"
            label="Metrics"
            value={metrics}
            error={error || metricsError}
            onEnter={handleApplyFilters}
            onChange={handleMetricsChangeInput}
            placeholder={"up{env=\"dev\"}\nup{env=\"prod\"}\n"}
          />
        </div>
        <div className="vm-retention-filters-body__result">
          <table className="vm-table">
            <thead className="vm-table-header">
              <tr>
                <th className="vm-table-cell vm-table-cell_header">Metric</th>
                <th className="vm-table-cell vm-table-cell_header">Applied retention</th>
              </tr>
            </thead>
            <tbody className="vm-table-body">
              {rows}
            </tbody>
          </table>
        </div>
        <div className="vm-retention-filters-body-top">
          <a
            className="vm-link vm-link_with-icon"
            target="_blank"
            href="https://docs.victoriametrics.com/#retention-filters"
            rel="help noreferrer"
          >
            <WikiIcon/>
            Documentation
          </a>
          <Button
            variant="text"
            onClick={handleRunExample}
          >
            Try example
          </Button>
          <Button
            variant="contained"
            onClick={handleApplyFilters}
            startIcon={<PlayIcon/>}
          >
            Apply
          </Button>
        </div>
      </div>
    </section>
  );
};

export default RetentionFilters;
