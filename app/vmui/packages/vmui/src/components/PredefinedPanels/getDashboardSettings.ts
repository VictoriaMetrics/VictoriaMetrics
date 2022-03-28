import {DashboardSettings} from "../../types";

const importModule = async (filename: string) => {
  const module = await import(`../../dashboards/${filename}`);
  module.default.filename = filename;
  return module.default as DashboardSettings;
};

export default async () => {
  const context = require.context("../../dashboards", true, /\.json$/);
  const filenames = context.keys().map(r => r.replace("./", ""));
  return await Promise.all(filenames.map(async f => importModule(f)));
};

