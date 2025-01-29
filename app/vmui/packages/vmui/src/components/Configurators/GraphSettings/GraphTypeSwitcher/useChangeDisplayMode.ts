import { useTimeDispatch } from "../../../../state/time/TimeStateContext";
import { useSearchParams } from "react-router-dom";

export const useChangeDisplayMode = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const dispatch = useTimeDispatch();

  const handleChange = (val: boolean, callback?: () => void) => {
    val ? searchParams.delete("display_mode") : searchParams.set("display_mode", "lines");
    setSearchParams(searchParams);
    dispatch({ type: "RUN_QUERY" });
    callback && callback();
  };

  return { handleChange };
};
