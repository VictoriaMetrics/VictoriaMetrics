export enum AppType {
  victoriametrics = "victoriametrics",
  victorialogs = "victorialogs",
  vmanomaly = "vmanomaly",
}

export const APP_TYPE = import.meta.env.VITE_APP_TYPE;
export const APP_TYPE_VM = APP_TYPE === AppType.victoriametrics;
export const APP_TYPE_LOGS = APP_TYPE === AppType.victorialogs;
export const APP_TYPE_ANOMALY = APP_TYPE === AppType.vmanomaly;


