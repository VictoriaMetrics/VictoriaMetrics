import React, { FC, useEffect, useMemo, useState } from "preact/compat";
import getDashboardSettings from "./getDashboardSettings";
import { DashboardRow, DashboardSettings } from "../../types";
import Box from "@mui/material/Box";
import Alert from "@mui/material/Alert";
import Tabs from "@mui/material/Tabs";
import Tab from "@mui/material/Tab";
import PredefinedDashboard from "./PredefinedDashboard";
import get from "lodash.get";
import { useSetQueryParams } from "./hooks/useSetQueryParams";

const DashboardLayout: FC = () => {
  useSetQueryParams();

  const [dashboards, setDashboards] = useState<DashboardSettings[]>();
  const [tab, setTab] = useState(0);

  const filename = useMemo(() => get(dashboards, [tab, "filename"], ""), [dashboards, tab]);

  const rows = useMemo(() => {
    return get(dashboards, [tab, "rows"], []) as DashboardRow[];
  }, [dashboards, tab]);

  useEffect(() => {
    getDashboardSettings().then(d => d.length && setDashboards(d));
  }, []);

  return <>
    {!dashboards && <Alert
      color="info"
      severity="info"
      sx={{ m: 4 }}
    >Dashboards not found</Alert>}
    {dashboards && <>
      <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
        <Tabs
          value={tab}
          onChange={(e, val) => setTab(val)}
          aria-label="dashboard-tabs"
        >
          {dashboards && dashboards.map((d, i) =>
            <Tab
              key={i}
              label={d.title || d.filename}
              id={`tab-${i}`}
              aria-controls={`tabpanel-${i}`}
            />
          )}
        </Tabs>
      </Box>
      <Box>
        {Array.isArray(rows) && !!rows.length
          ? rows.map((r,i) =>
            <PredefinedDashboard
              key={`${tab}_${i}`}
              index={i}
              filename={filename}
              title={r.title}
              panels={r.panels}
            />)
          : <Alert
            color="error"
            severity="error"
            sx={{ m: 4 }}
          >
            <code>&quot;rows&quot;</code> not found. Check the configuration file <b>{filename}</b>.
          </Alert>}
      </Box>
    </>}
  </>;
};

export default DashboardLayout;
