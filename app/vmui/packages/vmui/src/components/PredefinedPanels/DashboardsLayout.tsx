import React, {FC, useEffect, useMemo, useState} from "preact/compat";
import Header from "../Header/Header";
import getDashboardSettings from "./getDashboardSettings";
import {DashboardRow, DashboardSettings} from "../../types";
import Box from "@mui/material/Box";
import Alert from "@mui/material/Alert";
import Tabs from "@mui/material/Tabs";
import Tab from "@mui/material/Tab";
import PredefinedDashboard from "./PredefinedDashboard";
import get from "lodash.get";

const DashboardLayout: FC = () => {

  const [dashboards, setDashboards] = useState<DashboardSettings[]>();
  const [tab, setTab] = useState(0);

  const rows = useMemo(() => {
    return get(dashboards, [tab, "rows"], []) as DashboardRow[];
  }, [dashboards, tab]);

  useEffect(() => {
    getDashboardSettings().then(d => setDashboards(d));
  }, []);

  return <Box id="homeLayout">
    <Header/>
    {!dashboards && <Alert color="info" severity="info" sx={{m: 4}}>Dashboards not found</Alert>}
    <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
      <Tabs value={tab} onChange={(e, val) => setTab(val)} aria-label="dashboard-tabs">
        {dashboards && dashboards.map((d, i) =>
          <Tab key={i} label={d.title} id={`tab-${i}`} aria-controls={`tabpanel-${i}`}/>
        )}
      </Tabs>
    </Box>
    <Box>
      {!rows.length && <Alert color="error" severity="error" sx={{m: 4}}>
        Dashboard set up incorrectly. Check file <b>{get(dashboards, [tab, "filename"], "")}</b>.
      </Alert>}
      {rows && rows.map((r,i) =>
        <PredefinedDashboard
          key={`${tab}_${i}`}
          index={i}
          title={r.title}
          panels={r.panels}/>)}
    </Box>
  </Box>;
};

export default DashboardLayout;