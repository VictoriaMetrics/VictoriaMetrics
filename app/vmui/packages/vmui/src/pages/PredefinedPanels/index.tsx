import React, { FC, useMemo, useState } from "preact/compat";
import { useFetchDashboards } from "./getDashboardSettings";
import PredefinedDashboard from "./PredefinedDashboard/PredefinedDashboard";
import { useSetQueryParams } from "./hooks/useSetQueryParams";
import Tabs from "../../components/Main/Tabs/Tabs";
import Alert from "../../components/Main/Alert/Alert";
import "./style.scss";

const Index: FC = () => {
  useSetQueryParams();
  const { error, dashboardsSettings } = useFetchDashboards();
  const [tab, setTab] = useState("0");

  const tabs = useMemo(() => dashboardsSettings.map((d, i) => ({
    label: d.title || "",
    value: `${i}`,
    className: "vm-predefined-panels-tabs__tab"
  })), [dashboardsSettings]);

  const activeDashboard = useMemo(() => dashboardsSettings[+tab] || {}, [dashboardsSettings, tab]);
  const rows = useMemo(() => activeDashboard?.rows, [activeDashboard]);
  const filename = useMemo(() => activeDashboard.title || activeDashboard.filename || "", [activeDashboard]);
  const validDashboardRows = useMemo(() => Array.isArray(rows) && !!rows.length, [rows]);

  const handleChangeTab = (value: string) => {
    setTab(value);
  };

  return <div className="vm-predefined-panels">
    {error && <Alert variant="error">{error}</Alert>}
    {!dashboardsSettings.length && <Alert variant="info">Dashboards not found</Alert>}
    {tabs.length > 1 && (
      <div className="vm-predefined-panels-tabs vm-block vm-block_empty-padding">
        <Tabs
          activeItem={tab}
          items={tabs}
          onChange={handleChangeTab}
        />
      </div>
    )}
    <div className="vm-predefined-panels__dashboards">
      {validDashboardRows && (
        rows.map((r,i) =>
          <PredefinedDashboard
            key={`${tab}_${i}`}
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

export default Index;
