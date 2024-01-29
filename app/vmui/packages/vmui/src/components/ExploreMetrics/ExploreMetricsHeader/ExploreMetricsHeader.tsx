import React, { FC, useEffect, useMemo } from "preact/compat";
import Select from "../../Main/Select/Select";
import "./style.scss";
import { GRAPH_SIZES } from "../../../constants/graph";
import classNames from "classnames";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import useBoolean from "../../../hooks/useBoolean";
import { getFromStorage, saveToStorage } from "../../../utils/storage";
import Alert from "../../Main/Alert/Alert";
import Button from "../../Main/Button/Button";
import { CloseIcon, TipIcon } from "../../Main/Icons";
import Tooltip from "../../Main/Tooltip/Tooltip";

interface ExploreMetricsHeaderProps {
  jobs: string[]
  instances: string[]
  names: string[]
  job: string
  instance: string
  size: string
  selectedMetrics: string[]
  onChangeJob: (job: string) => void
  onChangeInstance: (instance: string) => void
  onToggleMetric: (name: string) => void
  onChangeSize: (sizeId: string) => void
}

const sizeOptions = GRAPH_SIZES.map(s => s.id);

const ExploreMetricsHeader: FC<ExploreMetricsHeaderProps> = ({
  jobs,
  instances,
  names,
  job,
  instance,
  size,
  selectedMetrics,
  onChangeJob,
  onChangeInstance,
  onToggleMetric,
  onChangeSize
}) => {
  const noInstanceText = useMemo(() => job ? "" : "No instances. Please select job", [job]);
  const noMetricsText = useMemo(() => job ? "" : "No metric names. Please select job", [job]);
  const { isMobile } = useDeviceDetect();

  const {
    value: showTips,
    toggle: toggleShowTips,
    setFalse: setHideTips,
  } = useBoolean(getFromStorage("EXPLORE_METRICS_TIPS") !== "false");

  useEffect(() => {
    saveToStorage("EXPLORE_METRICS_TIPS", `${showTips}`);
  }, [showTips]);

  return (
    <>
      <div
        className={classNames({
          "vm-explore-metrics-header": true,
          "vm-explore-metrics-header_mobile": isMobile,
          "vm-block": true,
          "vm-block_mobile": isMobile,
        })}
      >
        <div className="vm-explore-metrics-header__job">
          <Select
            value={job}
            list={jobs}
            label="Job"
            placeholder="Please select job"
            onChange={onChangeJob}
            autofocus={!job && !!jobs.length && !isMobile}
            searchable
          />
        </div>
        <div className="vm-explore-metrics-header__instance">
          <Select
            value={instance}
            list={instances}
            label="Instance"
            placeholder="Please select instance"
            onChange={onChangeInstance}
            noOptionsText={noInstanceText}
            clearable
            searchable
          />
        </div>
        <div className="vm-explore-metrics-header__size">
          <Select
            label="Size graphs"
            value={size}
            list={sizeOptions}
            onChange={onChangeSize}
          />
          <Tooltip title={`${showTips ? "Hide" : "Show"} tip`}>
            <Button
              variant="text"
              color={showTips ? "warning" : "gray"}
              startIcon={<TipIcon/>}
              onClick={toggleShowTips}
              ariaLabel="visibility tips"
            />
          </Tooltip>
        </div>
        <div className="vm-explore-metrics-header-metrics">
          <Select
            label={"Metrics"}
            value={selectedMetrics}
            list={names}
            placeholder="Search metric name"
            onChange={onToggleMetric}
            noOptionsText={noMetricsText}
            clearable
            searchable
          />
        </div>
      </div>

      {showTips && (
        <Alert variant={"warning"}>
          <div className="vm-explore-metrics-header-description">
            <p>
              Please note: this page is solely designed for exploring Prometheus metrics.
              Prometheus metrics always contain <code>job</code> and <code>instance</code> labels
              (see <a
                className="vm-link vm-link_colored"
                href="https://prometheus.io/docs/concepts/jobs_instances/"
              >these docs</a>), and this page relies on them as filters. <br/>
              Please use this page for Prometheus metrics only, in accordance with their naming conventions.
            </p>
            <Button
              variant="text"
              size="small"
              startIcon={<CloseIcon/>}
              onClick={setHideTips}
              ariaLabel="close tips"
            />
          </div>
        </Alert>
      )}
    </>
  );
};

export default ExploreMetricsHeader;
