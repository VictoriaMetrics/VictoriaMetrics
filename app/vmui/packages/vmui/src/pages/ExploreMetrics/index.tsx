import React, { FC, useEffect, useMemo, useState } from "preact/compat";
import { useSetQueryParams } from "./hooks/useSetQueryParams";
import { useFetchJobs } from "./hooks/useFetchJobs";
import Select from "../../components/Main/Select/Select";
import Spinner from "../../components/Main/Spinner/Spinner";
import Alert from "../../components/Main/Alert/Alert";
import { useFetchInstances } from "./hooks/useFetchInstances";
import { useFetchNames } from "./hooks/useFetchNames";
import "./style.scss";
import ExploreMetricItem from "./ExploreMetricItem/ExploreMetricItem";
import TextField from "../../components/Main/TextField/TextField";
import { CloseIcon, SearchIcon } from "../../components/Main/Icons";
import Switch from "../../components/Main/Switch/Switch";

const ExploreMetrics: FC = () => {
  useSetQueryParams();

  const [job, setJob] = useState("");
  const [instance, setInstance] = useState("");
  const [searchMetric, setSearchMetric] = useState("");
  const [openMetrics, setOpenMetrics] = useState<string[]>([]);
  const [onlyGraphs, setOnlyGraphs] = useState(false);

  const { jobs, isLoading: loadingJobs, error: errorJobs } = useFetchJobs();
  const { instances, isLoading: loadingInstances, error: errorInstances } = useFetchInstances(job);
  const { names, isLoading: loadingNames, error: errorNames } = useFetchNames(job, instance);

  const noInstanceText = useMemo(() => job ? "" : "No instances. Please select job", [job]);

  const metrics = useMemo(() => {
    const showMetrics = onlyGraphs ? names.filter((m) => openMetrics.includes(m)) : names;
    if (!searchMetric) return showMetrics;
    try {
      const regexp = new RegExp(searchMetric, "i");
      const found = showMetrics.filter((m) => regexp.test(m));
      return found.sort((a,b) => (a.match(regexp)?.index || 0) - (b.match(regexp)?.index || 0));
    } catch (e) {
      return [];
    }
  }, [names, searchMetric, openMetrics, onlyGraphs]);

  const isLoading = useMemo(() => {
    return loadingJobs || loadingInstances || loadingNames;
  }, [loadingJobs, loadingInstances, loadingNames]);

  const error = useMemo(() => {
    return errorJobs || errorInstances || errorNames;
  }, [errorJobs, errorInstances, errorNames]);

  const handleClearSearch = () => {
    setSearchMetric("");
  };

  const handleOpenMetric = (val: boolean, id: string) => {
    setOpenMetrics(prev => {
      if (!val) {
        return prev.filter(item => item !== id);
      }
      if (!prev.includes(id)) {
        return [...prev, id];
      }

      return prev;
    });
  };

  useEffect(() => {
    setInstance("");
  }, [job]);

  return (
    <div className="vm-explore-metrics">
      <div className="vm-explore-metrics-header vm-block">
        <div className="vm-explore-metrics-header-top">
          <Select
            value={job}
            list={jobs}
            label="Job"
            placeholder="Please select job"
            onChange={setJob}
            searchable
          />
          <Select
            value={instance}
            list={instances}
            label="Instance"
            placeholder="Please select instance"
            onChange={setInstance}
            noOptionsText={noInstanceText}
            clearable
            searchable
          />
          <div className="vm-explore-metrics-header-top__switch-graphs">
            <Switch
              label={"Show only opened metrics"}
              value={onlyGraphs}
              onChange={setOnlyGraphs}
            />
          </div>
        </div>
        <TextField
          autofocus
          label="Metric search"
          value={searchMetric}
          onChange={setSearchMetric}
          startIcon={<SearchIcon/>}
          endIcon={(
            <div
              className="vm-explore-metrics-header__clear-icon"
              onClick={handleClearSearch}
            >
              <CloseIcon/>
            </div>
          )}
        />
      </div>

      {isLoading && <Spinner />}
      {error && <Alert variant="error">{error}</Alert>}
      {!job && <Alert variant="info">Please select job to see list of metric names.</Alert>}
      {!metrics.length && onlyGraphs && job && (
        <Alert variant="info">
          Open graphs not found. Turn off &quot;Show only open metrics&quot; to see list of metric names.
        </Alert>
      )}
      <div className="vm-explore-metrics-body">
        {metrics.map((n) => (
          <ExploreMetricItem
            key={n}
            name={n}
            job={job}
            instance={instance}
            openMetrics={openMetrics}
            onOpen={handleOpenMetric}
          />
        ))}
      </div>
    </div>
  );
};

export default ExploreMetrics;
