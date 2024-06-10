import { useEffect, useState } from "preact/compat";
import { ErrorTypes } from "../types";
import { useAppState } from "../state/common/StateContext";
import { useTimeDispatch } from "../state/time/TimeStateContext";
import { getFromStorage } from "../utils/storage";
import dayjs from "dayjs";
import { getBrowserTimezone } from "../utils/time";

const disabledDefaultTimezone = Boolean(getFromStorage("DISABLED_DEFAULT_TIMEZONE"));

const useFetchDefaultTimezone = () => {
  const { serverUrl } = useAppState();
  const timeDispatch = useTimeDispatch();

  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>("");

  const setTimezone = (timezoneStr: string) => {
    const timezone = timezoneStr.toLowerCase() === "local" ? getBrowserTimezone().region : timezoneStr;
    try {
      dayjs().tz(timezone).isValid();
      timeDispatch({ type: "SET_DEFAULT_TIMEZONE", payload: timezone });
      if (disabledDefaultTimezone) return;
      timeDispatch({ type: "SET_TIMEZONE", payload: timezone });
    } catch (e) {
      if (e instanceof Error) setError(`${e.name}: ${e.message}`);
    }
  };

  const fetchDefaultTimezone = async () => {
    if (!serverUrl || process.env.REACT_APP_TYPE) return;
    setError("");
    setIsLoading(true);

    try {
      const response = await fetch(`${serverUrl}/vmui/timezone`);
      const resp = await response.json();

      if (response.ok) {
        setTimezone(resp.timezone);
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
    fetchDefaultTimezone();
  }, [serverUrl]);

  return { isLoading, error };
};

export default useFetchDefaultTimezone;

