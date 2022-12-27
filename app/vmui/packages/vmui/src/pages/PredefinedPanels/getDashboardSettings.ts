import { useEffect, useState } from "preact/compat";
import { DashboardSettings, ErrorTypes } from "../../types";
import { useAppState } from "../../state/common/StateContext";

const importModule = async (filename: string) => {
  const data = await fetch(`./dashboards/${filename}`);
  const json = await data.json();
  return json as DashboardSettings;
};

export const useFetchDashboards = (): {
  isLoading: boolean,
  error?: ErrorTypes | string,
  dashboardsSettings: DashboardSettings[],
} => {

  const { serverUrl } = useAppState();
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();
  const [dashboardsSettings, setDashboards] = useState<DashboardSettings[]>([]);

  const fetchLocalDashboards = async () => {
    const filenames = window.__VMUI_PREDEFINED_DASHBOARDS__;
    return await Promise.all(filenames.map(async f => importModule(f)));
  };


  const fetchRemoteDashboards = async () => {
    if (!serverUrl) return;
    setError("");
    setIsLoading(true);

    try {
      const response = await fetch(`${serverUrl}/vmui/custom-dashboards`);
      const resp = await response.json();

      if (response.ok) {
        const { dashboardsSettings } = resp;
        if (dashboardsSettings && dashboardsSettings.length > 0) {
          setDashboards((prevDash) => [...prevDash, ...dashboardsSettings]);
        }
        setIsLoading(false);
      } else {
        setError(resp.error);
        setIsLoading(false);
      }
    } catch (e) {
      setIsLoading(false);
      if (e instanceof Error) setError(`${e.name}: ${e.message}`);
    }
  };


  useEffect(() => {
    fetchLocalDashboards().then(d => d.length && setDashboards((prevDash) => [...prevDash, ...d]));
    fetchRemoteDashboards();
  }, [serverUrl]);

  return { dashboardsSettings, isLoading, error };
};

