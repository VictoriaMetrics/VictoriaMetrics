import { useEffect, useState } from "preact/compat";
import { useSearchParams } from "react-router-dom";
import { LegendQueryParams } from "../types";

const urlKey = LegendQueryParams.format;

export const useLegendFormat = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const valueFromUrl = searchParams.get(urlKey) || "";
  const [format, setFormat] = useState(valueFromUrl);

  const onChange = (value: string) => {
    setFormat(value);
  };

  const onApply = () => {
    if (format) {
      searchParams.set(urlKey, format);
    } else {
      searchParams.delete(urlKey);
    }

    setSearchParams(searchParams);
  };

  useEffect(() => {
    const value = searchParams.get(urlKey);
    if (value !== format) {
      setFormat(value || "");
    }
  }, [searchParams]);

  return {
    format,
    onChange,
    onApply
  };
};
