import React, { FC, useMemo, useState } from "preact/compat";
import PredefinedDashboard from "./PredefinedDashboard/PredefinedDashboard";
import { useSetQueryParams } from "./hooks/useSetQueryParams";
import Alert from "../../components/Main/Alert/Alert";
import classNames from "classnames";
import "./style.scss";
import { useDashboardsState } from "../../state/dashboards/DashboardsStateContext";
import Spinner from "../../components/Main/Spinner/Spinner";

const DashboardsLayout: FC = () => {
  useSetQueryParams();
  const { dashboardsSettings, dashboardsLoading, dashboardsError } = useDashboardsState();
  const [dashboard, setDashboard] = useState(0);

  const dashboards = useMemo(() => dashboardsSettings.map((d, i) => ({
    label: d.title || "",
    value: i,
  })), [dashboardsSettings]);

  const activeDashboard = useMemo(() => dashboardsSettings[dashboard] || {}, [dashboardsSettings, dashboard]);
  const rows = useMemo(() => activeDashboard?.rows, [activeDashboard]);
  const filename = useMemo(() => activeDashboard.title || activeDashboard.filename || "", [activeDashboard]);
  const validDashboardRows = useMemo(() => Array.isArray(rows) && !!rows.length, [rows]);

  const handleChangeDashboard = (value: number) => {
    setDashboard(value);
  };

  const createHandlerSelectDashboard = (value: number) => () => {
    handleChangeDashboard(value);
  };

  return <div className="vm-predefined-panels">
    {dashboardsLoading && <Spinner />}
    {dashboardsError && <Alert variant="error">{dashboardsError}</Alert>}
    {!dashboardsSettings.length && <Alert variant="info">Dashboards not found</Alert>}
    {dashboards.length > 1 && (
      <div className="vm-predefined-panels-tabs vm-block">
        {dashboards.map(tab => (
          <div
            key={tab.value}
            className={classNames({
              "vm-predefined-panels-tabs__tab": true,
              "vm-predefined-panels-tabs__tab_active": tab.value == dashboard
            })}
            onClick={createHandlerSelectDashboard(tab.value)}
          >
            {tab.label}
          </div>
        ))}
      </div>
    )}
    <div className="vm-predefined-panels__dashboards">
      {validDashboardRows && (
        rows.map((r,i) =>
          <PredefinedDashboard
            key={`${dashboard}_${i}`}
            index={i}
            filename={filename}
            title={r.title}
            panels={r.panels}
          />)
      )}
      {!!dashboardsSettings.length && !validDashboardRows && (
        <Alert variant="error">
          <code>&quot;rows&quot;</code> not found. Check the configuration file <b>{filename}</b>.
        </Alert>
      )}
    </div>
  </div>;
};

export default DashboardsLayout;
