export enum AppType {
  victoriametrics = "victoriametrics",
  victorialogs = "victorialogs",
  vmanomaly = "vmanomaly",
  vmalert = "vmalert",
}

export const APP_TYPE = import.meta.env.VITE_APP_TYPE;
export const APP_TYPE_VM = APP_TYPE === AppType.victoriametrics;
export const APP_TYPE_LOGS = APP_TYPE === AppType.victorialogs;
export const APP_TYPE_ANOMALY = APP_TYPE === AppType.vmanomaly;
export const APP_TYPE_ALERT = APP_TYPE === AppType.vmalert;

export const IsDefaultDatasourceType = (datasourceType: string): boolean => {
  switch (APP_TYPE) {
    case AppType.victorialogs:
      return "vlogs" == datasourceType;
    case AppType.victoriametrics:
      return datasourceType == "prometheus" || datasourceType == "";
    default:
      return false;
  }
};
