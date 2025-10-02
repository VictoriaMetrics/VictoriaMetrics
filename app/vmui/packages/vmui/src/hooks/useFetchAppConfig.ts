import { useAppDispatch, useAppState } from "../state/common/StateContext";
import { useEffect, useState } from "preact/compat";
import { ErrorTypes } from "../types";
import { APP_TYPE_VM } from "../constants/appType";

const useFetchAppConfig = () => {
  const { serverUrl } = useAppState();
  const dispatch = useAppDispatch();

  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>("");

  useEffect(() => {
    const fetchAppConfig = async () => {
      if (!APP_TYPE_VM) return;
      setError("");
      setIsLoading(true);

      try {
        const data = await fetch(`${serverUrl}/vmui/config.json`);
        const config = await data.json();
        dispatch({ type: "SET_APP_CONFIG", payload: config || {} });
      } catch (e) {
        setIsLoading(false);
        if (e instanceof Error) setError(`${e.name}: ${e.message}`);
      }
    };

    fetchAppConfig();
  }, [serverUrl]);

  return { isLoading, error };
};

export default useFetchAppConfig;

