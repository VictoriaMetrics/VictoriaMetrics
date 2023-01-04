import React, { FC, useEffect, useMemo, useState } from "preact/compat";
import { useSetQueryParams } from "./hooks/useSetQueryParams";
import { useFetchJobs } from "./hooks/useFetchJobs";
import Spinner from "../../components/Main/Spinner/Spinner";
import Alert from "../../components/Main/Alert/Alert";
import { useFetchInstances } from "./hooks/useFetchInstances";
import { useFetchNames } from "./hooks/useFetchNames";
import "./style.scss";
import ExploreMetricItem from "../../components/ExploreMetrics/ExploreMetricItem/ExploreMetricItem";
import ExploreMetricsHeader from "../../components/ExploreMetrics/ExploreMetricsHeader/ExploreMetricsHeader";

const ExploreMetrics: FC = () => {
  useSetQueryParams();

  const [job, setJob] = useState("");
  const [instance, setInstance] = useState("");
  const [metrics, setMetrics] = useState<string[]>([]);

  const { jobs, isLoading: loadingJobs, error: errorJobs } = useFetchJobs();
  const { instances, isLoading: loadingInstances, error: errorInstances } = useFetchInstances(job);
  const { names, isLoading: loadingNames, error: errorNames } = useFetchNames(job, instance);

  const isLoading = useMemo(() => {
    return loadingJobs || loadingInstances || loadingNames;
  }, [loadingJobs, loadingInstances, loadingNames]);

  const error = useMemo(() => {
    return errorJobs || errorInstances || errorNames;
  }, [errorJobs, errorInstances, errorNames]);

  const handleToggleMetric = (name: string) => {
    if (!name) {
      setMetrics([]);
    } else {
      setMetrics((prev) => prev.includes(name) ? prev.filter(n => n !== name) : [...prev, name]);
    }
  };

  const handleChangeOrder = (name: string, oldIndex: number, newIndex: number) => {
    const maxIndex = newIndex > (metrics.length - 1);
    const minIndex = newIndex < 0;
    if (minIndex || maxIndex) return;
    setMetrics(prev => {
      const updatedList = [...prev];
      const [reorderedItem] = updatedList.splice(oldIndex, 1);
      updatedList.splice(newIndex, 0, reorderedItem);
      return updatedList;
    });
  };

  useEffect(() => {
    setInstance("");
  }, [job]);

  return (
    <div className="vm-explore-metrics">
      <ExploreMetricsHeader
        jobs={jobs}
        instances={instances}
        names={names}
        job={job}
        instance={instance}
        selectedMetrics={metrics}
        onChangeJob={setJob}
        onChangeInstance={setInstance}
        onToggleMetric={handleToggleMetric}
      />

      {isLoading && <Spinner />}
      {error && <Alert variant="error">{error}</Alert>}
      {!job && <Alert variant="info">Please select job to see list of metric names.</Alert>}
      {job && !metrics.length && <Alert variant="info">Please select metric names to see the graphs.</Alert>}
      <div className="vm-explore-metrics-body">
        {metrics.map((n, i) => (
          <ExploreMetricItem
            key={n}
            name={n}
            job={job}
            instance={instance}
            index={i}
            onRemoveItem={handleToggleMetric}
            onChangeOrder={handleChangeOrder}
          />
        ))}
      </div>
    </div>
  );
};

export default ExploreMetrics;
