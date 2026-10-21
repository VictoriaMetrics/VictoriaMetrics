import { FC, useMemo } from "preact/compat";
import { useFetchActiveQueries } from "./hooks/useFetchActiveQueries";
import Alert from "../../components/Main/Alert/Alert";
import Spinner from "../../components/Main/Spinner/Spinner";
import Table, { TableColumn } from "../../components/Table/Table";
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
import { formatTenant } from "../../utils/tenants";

const ActiveQueries: FC = () => {
  const { isMobile } = useDeviceDetect();
  const { timezone } = useTimeState();

  const { data, lastUpdated, isLoading, error, fetchData } = useFetchActiveQueries();

  const activeQueries = useMemo(() => data.map((item: ActiveQueriesType) => {
    const from = dayjs(item.start).tz().format(DATE_TIME_FORMAT);
    const to = dayjs(item.end).tz().format(DATE_TIME_FORMAT);

    const accountID = Number(item.account_id);
    const projectID = Number(item.project_id);

    return {
      duration: item.duration,
      remote_addr: item.remote_addr,
      query: item.query,
      args: `${from} to ${to}, step=${roundStep(item.step)}`,
      data: JSON.stringify(item, null, 2),
      // Normalize tenant fields
      multiTenant: item.is_multitenant,
      accountID: Number.isFinite(accountID) ? accountID : undefined,
      projectID: Number.isFinite(projectID) ? projectID : undefined,
    } as ActiveQueriesType;
  }), [data, timezone]);

  const tenantColumn: TableColumn<ActiveQueriesType> | null = useMemo(() => {
    if (!data?.length) return null;

    const someHasTenant = activeQueries.some(q => !!formatTenant(q));
    if (!someHasTenant) return null;

    return {
      key: "accountID",
      title: "tenant",
      format: (q: ActiveQueriesType) => formatTenant(q) || ""
    };
  }, [activeQueries]);

  const columns: TableColumn<ActiveQueriesType>[] = useMemo(() => {
    if (!activeQueries?.length) return [];
    const keys = Object.keys(activeQueries[0]) as (keyof ActiveQueriesType)[];

    const titles: Partial<Record<keyof ActiveQueriesType, string>> = {
      remote_addr: "client address",
    };
    const hideColumns = ["data", "multiTenant", "accountID", "projectID"];

    const baseCols = keys.filter((col) => !hideColumns.includes(col)).map((key) => ({
      key: key,
      title: titles[key] || key,
    }));

    if (tenantColumn) return [...baseCols, tenantColumn];

    return baseCols;
  }, [activeQueries, tenantColumn]);

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
