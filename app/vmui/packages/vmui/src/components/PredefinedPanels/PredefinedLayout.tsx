import React, {FC, useEffect, useState} from "preact/compat";
import Header from "../Header/Header";
import getDashboardSettings from "./getDashboardSettings";
import {DashboardSettings} from "../../types";
import Box from "@mui/material/Box";
import Alert from "@mui/material/Alert";
import Divider from "@mui/material/Divider";
import PredefinedDashboard from "./PredefinedDashboard";

const PredefinedLayout: FC = () => {

  const [dashboardSettings, setDashboardSettings] = useState<DashboardSettings[]>();

  useEffect(() => {
    getDashboardSettings().then(settings => setDashboardSettings(settings));
  }, []);

  return <Box id="homeLayout">
    <Header/>
    {!dashboardSettings && <Alert color="info" severity="info" sx={{fontSize: "14px", m: 4}}>
      Predefined panels not found
    </Alert>}
    {dashboardSettings && dashboardSettings.map((d, i) =>
      <Box key={d.filename}>
        <PredefinedDashboard index={i} name={d.name} panels={d.panels} filename={d.filename}/>
        <Divider/>
      </Box>
    )}
  </Box>;
};

export default PredefinedLayout;