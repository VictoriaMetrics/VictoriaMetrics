import React, { FC, useMemo } from "preact/compat";
import "./style.scss";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import classNames from "classnames";
import { LogHits } from "../../../api/types";
import dayjs from "dayjs";
import { useTimeDispatch } from "../../../state/time/TimeStateContext";
import { AlignedData } from "uplot";
import BarHitsChart from "../../../components/Chart/BarHitsChart/BarHitsChart";
import Alert from "../../../components/Main/Alert/Alert";
import { TimeParams } from "../../../types";
import LineLoader from "../../../components/Main/LineLoader/LineLoader";

interface Props {
  query: string;
  logHits: LogHits[];
  period: TimeParams;
  error?: string;
  isLoading: boolean;
  onApplyFilter: (value: string) => void;
}

const ExploreLogsBarChart: FC<Props> = ({ logHits, period, error, isLoading, onApplyFilter }) => {
  const { isMobile } = useDeviceDetect();
  const timeDispatch = useTimeDispatch();

  const getXAxis = (timestamps: string[]): number[] => {
    return (timestamps.map(t => t ? dayjs(t).unix() : null)
      .filter(Boolean) as number[])
      .sort((a, b) => a - b);
  };

  const getYAxes = (logHits: LogHits[], timestamps: string[]) => {
    return logHits.map(hits => {
      return timestamps.map(t => {
        const index = hits.timestamps.findIndex(ts => ts === t);
        return index === -1 ? null : hits.values[index] || null;
      });
    });
  };

  const data = useMemo(() => {
    if (!logHits.length) return [[], []] as AlignedData;
    const timestamps = Array.from(new Set(logHits.map(l => l.timestamps).flat()));
    const xAxis = getXAxis(timestamps);
    const yAxes = getYAxes(logHits, timestamps);
    return [xAxis, ...yAxes] as AlignedData;
  }, [logHits]);

  const noDataMessage: string = useMemo(() => {
    const noData = data.every(d => d.length === 0);
    const noTimestamps = data[0].length === 0;
    const noValues = data[1].length === 0;
    if (noData) {
      return "No logs volume available\nNo volume information available for the current queries and time range.";
    } else if (noTimestamps) {
      return "No timestamp information available for the current queries and time range.";
    } else if (noValues) {
      return "No value information available for the current queries and time range.";
    } return "";
  }, [data]);

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
      {isLoading && <LineLoader/>}
      {!error && noDataMessage && (
        <div className="vm-explore-logs-chart__empty">
          <Alert variant="info">{noDataMessage}</Alert>
        </div>
      )}

      {error && noDataMessage && (
        <div className="vm-explore-logs-chart__empty">
          <Alert variant="error">{error}</Alert>
        </div>
      )}

      {data && (
        <BarHitsChart
          logHits={logHits}
          data={data}
          period={period}
          setPeriod={setPeriod}
          onApplyFilter={onApplyFilter}
        />
      )}
    </section>
  );
};

export default ExploreLogsBarChart;
