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

const ExploreMetrics: FC = () => {
  useSetQueryParams();

  const [job, setJob] = useState("");
  const [instance, setInstance] = useState("");

  const { jobs, isLoading: loadingJobs, error: errorJobs } = useFetchJobs();
  const { instances, isLoading: loadingInstances, error: errorInstances } = useFetchInstances(job);
  const { names, isLoading: loadingNames, error: errorNames } = useFetchNames(job);

  const isLoading = useMemo(() => {
    return loadingJobs || loadingInstances || loadingNames;
  }, [loadingJobs, loadingInstances, loadingNames]);

  const error = useMemo(() => {
    return errorJobs || errorInstances || errorNames;
  }, [errorJobs, errorInstances, errorNames]);

  useEffect(() => {
    setInstance("");
  }, [job]);

  return (
    <div className="vm-explore-metrics">
      <div className="vm-explore-metrics-header vm-block">
        <Select
          value={job}
          list={jobs}
          label="Job"
          placeholder="Please select job"
          onChange={setJob}
        />
        <Select
          value={instance}
          list={instances}
          label="Instance"
          placeholder="Please select instance"
          onChange={setInstance}
          noOptionsText="No instances. Please select job"
          clearable
        />
      </div>

      {isLoading && <Spinner />}
      {error && <Alert variant="error">{error}</Alert>}
      <div className="vm-explore-metrics-body">
        {names.map((n, i) => (
          <ExploreMetricItem
            key={`${n}_${i}`}
            name={n}
            job={job}
            instance={instance}
          />
        ))}
      </div>
    </div>
  );
};

export default ExploreMetrics;
