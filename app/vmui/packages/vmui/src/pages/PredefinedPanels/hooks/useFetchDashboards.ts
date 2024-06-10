import { useEffect, useState } from "preact/compat";
import { DashboardSettings, ErrorTypes } from "../../../types";
import { useAppState } from "../../../state/common/StateContext";
import { useDashboardsDispatch } from "../../../state/dashboards/DashboardsStateContext";
import { getAppModeEnable } from "../../../utils/app-mode";

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
  const appModeEnable = getAppModeEnable();
  const { serverUrl } = useAppState();
  const dispatch = useDashboardsDispatch();
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>("");
  const [dashboardsSettings, setDashboards] = useState<DashboardSettings[]>([]);

  const fetchLocalDashboards = async () => {
    try {
      const filenames = window.__VMUI_PREDEFINED_DASHBOARDS__;
      if (!filenames?.length) return [];
      const dashboards = await Promise.all(filenames.map(async f => importModule(f)));
      setDashboards((prevDash) => [...dashboards, ...prevDash]);
    } catch (e) {
      if (e instanceof Error) setError(`${e.name}: ${e.message}`);
    }
  };

  const fetchRemoteDashboards = async () => {
    if (!serverUrl || process.env.REACT_APP_TYPE) return;
    setError("");
    setIsLoading(true);

    try {
      const response = await fetch(`${serverUrl}/vmui/custom-dashboards`);
      const resp = await response.json();

      if (response.ok) {
        const { dashboardsSettings } = resp;
        if (dashboardsSettings && dashboardsSettings.length > 0) {
          setDashboards((prevDash) => [...prevDash, ...dashboardsSettings]);
        } else {
          await fetchLocalDashboards();
        }
        setIsLoading(false);
      } else {
        await fetchLocalDashboards();
        setError(resp.error);
        setIsLoading(false);
      }
    } catch (e) {
      setIsLoading(false);
      if (e instanceof Error) setError(`${e.name}: ${e.message}`);
      await fetchLocalDashboards();
    }
  };

  useEffect(() => {
    if (appModeEnable) return;
    setDashboards([]);
    fetchRemoteDashboards();
  }, [serverUrl]);

  useEffect(() => {
    dispatch({ type: "SET_DASHBOARDS_SETTINGS", payload: dashboardsSettings });
  }, [dashboardsSettings]);

  useEffect(() => {
    dispatch({ type: "SET_DASHBOARDS_LOADING", payload: isLoading });
  }, [isLoading]);

  useEffect(() => {
    dispatch({ type: "SET_DASHBOARDS_ERROR", payload: error });
  }, [error]);

  return { dashboardsSettings, isLoading, error };
};

