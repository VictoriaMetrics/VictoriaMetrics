import React, { FC, useMemo } from "preact/compat";
import "./style.scss";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import classNames from "classnames";
import { LogHits } from "../../../api/types";
import dayjs from "dayjs";
import { useTimeDispatch, useTimeState } from "../../../state/time/TimeStateContext";
import { AlignedData } from "uplot";
import BarHitsChart from "../../../components/Chart/BarHitsChart/BarHitsChart";
import Alert from "../../../components/Main/Alert/Alert";

interface Props {
  query: string;
  logHits: LogHits[];
  error?: string;
  isLoading: boolean;
  loaded: boolean;
}

const ExploreLogsBarChart: FC<Props> = ({ logHits, error, loaded }) => {
  const { isMobile } = useDeviceDetect();
  const { period } = useTimeState();
  const timeDispatch = useTimeDispatch();

  const data = useMemo(() => {
    const hits = logHits[0];
    if (!hits) return [[], []] as AlignedData;
    const { values, timestamps } = hits;
    const xAxis = timestamps.map(t => t ? dayjs(t).unix() : null).filter(Boolean);
    const yAxis = values.map(v => v || null);
    return [xAxis, yAxis] as AlignedData;
  }, [logHits]);

  const noData = data.some(d => d.length === 0);

  const setPeriod = ({ from, to }: {from: Date, to: Date}) => {
    timeDispatch({ type: "SET_PERIOD", payload: { from, to } });
  };

  return (
    <section
      className={classNames({
        "vm-explore-logs-chart": true,
        "vm-block": true,
        "vm-block_mobile": isMobile,
      })}
    >
      {!error && loaded && noData && (
        <div className="vm-explore-logs-chart__empty">
          <Alert variant="info">
            <p>No logs volume available</p>
            <p>No volume information available for the current queries and time range.</p>
          </Alert>
        </div>
      )}

      {error && loaded && noData && (
        <div className="vm-explore-logs-chart__empty">
          <Alert variant="error">{error}</Alert>
        </div>
      )}

      {data && (
        <BarHitsChart
          data={data}
          period={period}
          setPeriod={setPeriod}
        />
      )}
    </section>
  );
};

export default ExploreLogsBarChart;
