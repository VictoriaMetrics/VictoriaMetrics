import React, { FC, useEffect, useMemo, useState } from "preact/compat";
import getDashboardSettings from "./getDashboardSettings";
import { DashboardRow, DashboardSettings } from "../../types";
import PredefinedDashboard from "./PredefinedDashboard/PredefinedDashboard";
import get from "lodash.get";
import { useSetQueryParams } from "./hooks/useSetQueryParams";

const Index: FC = () => {
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
    {/*{!dashboards && <Alert*/}
    {/*  color="info"*/}
    {/*  severity="info"*/}
    {/*  sx={{ m: 4 }}*/}
    {/*>Dashboards not found</Alert>}*/}
    {dashboards && <>
      <div>
        {/* TODO add tabs */}
        <div>
          {dashboards && dashboards.map((d, i) =>
            <div
              key={i}
              id={`tab-${i}`}
            >
              {d.title || d.filename}
            </div>
          )}
        </div>
      </div>
      <div>
        {Array.isArray(rows) && !!rows.length
          ? rows.map((r,i) =>
            <PredefinedDashboard
              key={`${tab}_${i}`}
              index={i}
              filename={filename}
              title={r.title}
              panels={r.panels}
            />)
          : (
            <div>error</div>
            // <Alert
            //   color="error"
            //   severity="error"
            //   sx={{ m: 4 }}
            // >
            //   <code>&quot;rows&quot;</code> not found. Check the configuration file <b>{filename}</b>.
            // </Alert>
          )}
      </div>
    </>}
  </>;
};

export default Index;
