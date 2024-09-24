import { useAppDispatch, useAppState } from "../state/common/StateContext";
import { useEffect, useState } from "preact/compat";
import { ErrorTypes } from "../types";
import { getUrlWithoutTenant } from "../utils/tenants";

const useFetchFlags = () => {
  const { serverUrl } = useAppState();
  const dispatch = useAppDispatch();

  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>("");

  useEffect(() => {
    const fetchFlags = async () => {
      if (!serverUrl || process.env.REACT_APP_TYPE) return;
      setError("");
      setIsLoading(true);

      try {
        const url = getUrlWithoutTenant(serverUrl);
        const response = await fetch(`${url}/flags`);
        const data = await response.text();
        const flags = data.split("\n").filter(flag => flag.trim() !== "")
          .reduce((acc, flag) => {
            const [keyRaw, valueRaw] = flag.split("=");
            const key = keyRaw.trim().replace(/^-/, "");
            acc[key.trim()] = valueRaw ? valueRaw.trim().replace(/^"(.*)"$/, "$1") : null;
            return acc;
          }, {} as Record<string, string|null>);
        dispatch({ type: "SET_FLAGS", payload: flags });
      } catch (e) {
        setIsLoading(false);
        if (e instanceof Error) setError(`${e.name}: ${e.message}`);
      }
    };

    fetchFlags();
  }, [serverUrl]);

  return { isLoading, error };
};

export default useFetchFlags;

