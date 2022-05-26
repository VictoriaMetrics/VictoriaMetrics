import {DashboardSettings} from "../../types";

const importModule = async (filename: string) => {
  const data = await fetch(`./dashboards/${filename}`);
  const json = await data.json();
  return json as DashboardSettings;
};

export default async () => {
  const filenames = window.__VMUI_PREDEFINED_DASHBOARDS__;
  return await Promise.all(filenames.map(async f => importModule(f)));
};
