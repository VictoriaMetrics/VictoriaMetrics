import {DashboardSettings} from "../../types";

const importModule = async (filename: string) => {
  const module = await import(`../../predefined-panels/${filename}`);
  module.default.filename = filename.replace(/\.json$/, "");
  return module.default as DashboardSettings;
};

export default async () => {
  const context = require.context("../../predefined-panels", false, /\.json$/);
  const filenames = context.keys().map(r => r.replace("./", ""));
  return await Promise.all(filenames.map(async f => importModule(f)));
};

