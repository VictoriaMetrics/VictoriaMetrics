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
import { GRAPH_SIZES } from "../../constants/graph";
import { getQueryStringValue } from "../../utils/query-string";

const defaultJob = getQueryStringValue("job", "") as string;
const defaultInstance = getQueryStringValue("instance", "") as string;
const defaultMetricsStr = getQueryStringValue("metrics", "") as string;
const defaultSizeId = getQueryStringValue("size", "") as string;
const defaultSize = GRAPH_SIZES.find(v => defaultSizeId ? v.id === defaultSizeId : v.isDefault) || GRAPH_SIZES[0];

const ExploreMetrics: FC = () => {
  const [job, setJob] = useState(defaultJob);
  const [instance, setInstance] = useState(defaultInstance);
  const [metrics, setMetrics] = useState(defaultMetricsStr ? defaultMetricsStr.split("&") : []);
  const [size, setSize] = useState(defaultSize);

  useSetQueryParams({ job, instance, metrics: metrics.join("&"), size: size.id });

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

  const handleChangeSize = (sizeId: string) => {
    const target = GRAPH_SIZES.find(variant => variant.id === sizeId);
    if (target) setSize(target);
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
    if (instance && instances.length && !instances.includes(instance)) {
      setInstance("");
    }
  }, [instances, instance]);

  return (
    <div className="vm-explore-metrics">
      <ExploreMetricsHeader
        jobs={jobs}
        instances={instances}
        names={names}
        job={job}
        size={size.id}
        instance={instance}
        selectedMetrics={metrics}
        onChangeJob={setJob}
        onChangeSize={handleChangeSize}
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
            length={metrics.length}
            size={size}
            onRemoveItem={handleToggleMetric}
            onChangeOrder={handleChangeOrder}
          />
        ))}
      </div>
    </div>
  );
};

export default ExploreMetrics;
