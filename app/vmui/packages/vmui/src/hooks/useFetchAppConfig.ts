import { useAppDispatch } from "../state/common/StateContext";
import { useEffect, useState } from "preact/compat";
import { ErrorTypes } from "../types";
import { APP_TYPE_VM } from "../constants/appType";

const useFetchFlags = () => {
  const dispatch = useAppDispatch();

  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>("");

  useEffect(() => {
    const fetchAppConfig = async () => {
      if (!APP_TYPE_VM) return;
      setError("");
      setIsLoading(true);

      try {
        const data = await fetch("./config.json");
        const config = await data.json();
        dispatch({ type: "SET_APP_CONFIG", payload: config || {} });
      } catch (e) {
        setIsLoading(false);
        if (e instanceof Error) setError(`${e.name}: ${e.message}`);
      }
    };

    fetchAppConfig();
  }, []);

  return { isLoading, error };
};

export default useFetchFlags;

