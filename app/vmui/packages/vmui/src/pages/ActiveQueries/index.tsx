import React, { FC, useMemo } from "preact/compat";
import { useFetchActiveQueries } from "./hooks/useFetchActiveQueries";
import Alert from "../../components/Main/Alert/Alert";
import Spinner from "../../components/Main/Spinner/Spinner";
import Table from "../../components/Table/Table";
import { ActiveQueriesType } from "../../types";
import dayjs from "dayjs";
import { useTimeState } from "../../state/time/TimeStateContext";
import useDeviceDetect from "../../hooks/useDeviceDetect";
import classNames from "classnames";
import Button from "../../components/Main/Button/Button";
import { RefreshIcon } from "../../components/Main/Icons";
import "./style.scss";
import { DATE_TIME_FORMAT } from "../../constants/date";
import { roundStep } from "../../utils/time";

const ActiveQueries: FC = () => {
  const { isMobile } = useDeviceDetect();
  const { timezone } = useTimeState();

  const { data, lastUpdated, isLoading, error, fetchData } = useFetchActiveQueries();

  const activeQueries = useMemo(() => data.map((item: ActiveQueriesType) => {
    const from = dayjs(item.start).tz().format(DATE_TIME_FORMAT);
    const to = dayjs(item.end).tz().format(DATE_TIME_FORMAT);
    return {
      duration: item.duration,
      remote_addr: item.remote_addr,
      query: item.query,
      args: `${from} to ${to}, step=${roundStep(item.step)}`,
      data: JSON.stringify(item, null, 2),
    } as ActiveQueriesType;
  }), [data, timezone]);

  const columns = useMemo(() => {
    if (!activeQueries?.length) return [];
    const keys = Object.keys(activeQueries[0]) as (keyof ActiveQueriesType)[];

    const titles: Partial<Record<keyof ActiveQueriesType, string>> = {
      remote_addr: "client address",
    };
    const hideColumns = ["data"];

    return keys.filter((col) => !hideColumns.includes(col)).map((key) => ({
      key: key,
      title: titles[key] || key,
    }));
  }, [activeQueries]);

  const handleRefresh = async () => {
    fetchData().catch(console.error);
  };

  return (
    <div className="vm-active-queries">
      {isLoading && <Spinner />}
      <div className="vm-active-queries-header">
        {!activeQueries.length && !error && <Alert variant="info">There are currently no active queries running</Alert>}
        {error && <Alert variant="error">{error}</Alert>}
        <div className="vm-active-queries-header-controls">
          <Button
            variant="contained"
            onClick={handleRefresh}
            startIcon={<RefreshIcon/>}
          >
            Update
          </Button>
          <div className="vm-active-queries-header__update-msg">
            Last updated: {lastUpdated}
          </div>
        </div>
      </div>
      {!!activeQueries.length && (
        <div
          className={classNames({
            "vm-block":  true,
            "vm-block_mobile": isMobile,
          })}
        >
          <Table
            rows={activeQueries}
            columns={columns}
            defaultOrderBy={"duration"}
            copyToClipboard={"data"}
            paginationOffset={{ startIndex: 0, endIndex: Infinity }}
          />
        </div>
      )}
    </div>
  );
};

export default ActiveQueries;
