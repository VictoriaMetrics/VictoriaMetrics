import React, { FC, useMemo, useState } from "preact/compat";
import classNames from "classnames";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import Select from "../../../components/Main/Select/Select";
import "./style.scss";
import usePrevious from "../../../hooks/usePrevious";
import { useEffect } from "react";
import { arrayEquals } from "../../../utils/array";
import { getQueryStringValue } from "../../../utils/query-string";
import { useSetQueryParams } from "../hooks/useSetQueryParams";

type Props = {
  queries: string[];
  series?: Record<string, {[p: string]: string}[]>
  onChange: (expr: Record<string, string>) => void;
}

const ExploreAnomalyHeader: FC<Props> = ({ queries, series, onChange }) => {
  const { isMobile } = useDeviceDetect();
  const [alias, setAlias] = useState(queries[0]);
  const [selectedValues, setSelectedValues] = useState<Record<string, string>>({});
  useSetQueryParams({ alias: alias, ...selectedValues });

  const uniqueKeysWithValues = useMemo(() => {
    if (!series) return {};
    return series[alias]?.reduce((accumulator, currentSeries) => {
      const metric = Object.entries(currentSeries);
      if (!metric.length) return accumulator;
      const excludeMetrics = ["__name__", "for"];
      for (const [key, value] of metric) {
        if (excludeMetrics.includes(key) || accumulator[key]?.includes(value)) continue;

        if (!accumulator[key]) {
          accumulator[key] = [];
        }

        accumulator[key].push(value);
      }
      return accumulator;
    }, {} as Record<string, string[]>) || {};
  }, [alias, series]);
  const prevUniqueKeysWithValues = usePrevious(uniqueKeysWithValues);

  const createHandlerChangeSelect = (key: string) => (value: string) => {
    setSelectedValues((prev) => ({ ...prev, [key]: value }));
  };

  useEffect(() => {
    const nextValues = Object.values(uniqueKeysWithValues).flat();
    const prevValues = Object.values(prevUniqueKeysWithValues || {}).flat();
    if (arrayEquals(prevValues, nextValues)) return;
    const newSelectedValues: Record<string, string> = {};
    Object.keys(uniqueKeysWithValues).forEach((key) => {
      const value = getQueryStringValue(key, "") as string;
      newSelectedValues[key] = value || uniqueKeysWithValues[key]?.[0];
    });
    setSelectedValues(newSelectedValues);
  }, [uniqueKeysWithValues, prevUniqueKeysWithValues]);

  useEffect(() => {
    if (!alias || !Object.keys(selectedValues).length) return;
    const __name__ = series?.[alias]?.[0]?.__name__ || "";
    onChange({ ...selectedValues, for: alias, __name__ });
  }, [selectedValues, alias]);

  useEffect(() => {
    setAlias(getQueryStringValue("alias", queries[0]) as string);
  }, [series]);

  return (
    <div
      id="legendAnomaly"
      className={classNames({
        "vm-explore-anomaly-header": true,
        "vm-explore-anomaly-header_mobile": isMobile,
        "vm-block": true,
        "vm-block_mobile": isMobile,
      })}
    >
      <div className="vm-explore-anomaly-header-main">
        <div className="vm-explore-anomaly-header__select">
          <Select
            value={alias}
            list={queries}
            label="Query"
            placeholder="Please select query"
            onChange={setAlias}
            searchable
          />
        </div>
      </div>
      {Object.entries(uniqueKeysWithValues).map(([key, values]) => (
        <div
          className="vm-explore-anomaly-header__values"
          key={key}
        >
          <Select
            value={selectedValues[key] || ""}
            list={values}
            label={key}
            placeholder={`Please select ${key}`}
            onChange={createHandlerChangeSelect(key)}
            searchable={values.length > 2}
            disabled={values.length === 1}
          />
        </div>
      ))}
    </div>
  );
};

export default ExploreAnomalyHeader;
