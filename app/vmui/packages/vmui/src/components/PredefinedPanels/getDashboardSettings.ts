import {DashboardSettings} from "../../types";

export default (): DashboardSettings[] => {
  return window.__VMUI_PREDEFINED_DASHBOARDS__ || [];
};

