import React, { FC, useEffect, useMemo, useState } from "preact/compat";
import { Logs } from "../../../../api/types";
import { extractUniqueValues, parseStreamToObject } from "./utils";
import FilterBuilderLogs from "./FilterBuilderLogs";
import "./style.scss";
import Button from "../../../../components/Main/Button/Button";
import { FilterClearIcon, PlusIcon } from "../../../../components/Main/Icons";
import Toggle from "../../../../components/Main/Toggle/Toggle";
import qs from "qs";
import useStateSearchParams from "../../../../hooks/useStateSearchParams";
import { useSearchParams } from "react-router-dom";

interface Props {
  logs: Logs[];
  filtersFromParams: FilterObj[];
  onChange: (logs: Logs[]) => void;
}

export enum FilterType {
  stream = "_stream",
  extra = "extra fields",
}

export enum FilterOperator {
  AND = "AND",
  OR = "OR"
}

export interface FilterObj {
  id: number;
  type: FilterType;
  field: string;
  value: string;
}

const generateNewFilter = () => ({
  id: Date.now(),
  type: FilterType.stream,
  field: "",
  value: "",
});

const FilterLogs: FC<Props> = ({ logs, filtersFromParams, onChange }) => {
  const [searchParams, setSearchParams] = useSearchParams();

  const [operator, setOperator] = useStateSearchParams(FilterOperator.AND, "operator");
  const [filters, setFilters] = useState<FilterObj[]>([generateNewFilter()]);

  const uniqExtraValues = useMemo(() => {
    const excludeColumns = ["_msg", "time", "data", "_time", "_stream"];
    return extractUniqueValues(logs, log => log, excludeColumns);
  }, [logs]);

  const uniqStreamValues = useMemo(() => {
    return extractUniqueValues(logs, log => parseStreamToObject(log._stream));
  }, [logs]);

  const handleAddFilter = () => {
    setFilters(prev => [...prev, generateNewFilter()]);
  };

  const handleClearFilters = () => {
    setFilters([generateNewFilter()]);
  };

  const handleRemoveFilter = (id: number) => {
    setFilters(prev => {
      const filteredFilters = prev.filter(f => f.id !== id);
      return filteredFilters.length ? filteredFilters : [generateNewFilter()];
    });
  };

  const handleChangeFilter = (newValue: FilterObj) => {
    setFilters(prev => prev.map(filter => {
      if (filter.id !== newValue.id) return filter;
      return newValue;
    }));
  };

  const filteredLogs = useMemo(() => {
    const selectedFilters = filters.filter(f => f.value && f.field && f.type);
    if (!selectedFilters.length) return logs;
    const streamFilters = selectedFilters.filter(f => f.type === "_stream").map(f => `${f.field}="${f.value}"`);
    const extraFieldsFilters = selectedFilters.filter(f => f.type === "extra fields");

    if (operator === FilterOperator.OR) {
      return logs.filter(log => {
        const matchesStream = streamFilters.length > 0 && streamFilters.some(f => log._stream.includes(f));
        const matchesExtra = extraFieldsFilters.length > 0 && extraFieldsFilters.some(f => log[f.field] === f.value);
        return matchesStream || matchesExtra;
      });
    }

    const filteredByStreams = logs.filter(log =>
      !streamFilters.length || streamFilters.every(filter => log._stream.includes(filter))
    );

    return filteredByStreams.filter(log =>
      !extraFieldsFilters.length || extraFieldsFilters.every(filter => log[filter.field] === filter.value)
    );
  }, [filters, logs, operator]);

  useEffect(() => {
    onChange(filteredLogs);
  }, [filteredLogs]);

  useEffect(() => {
    const selectedFilters = filters.filter(f => f.value && f.field && f.type).map(({ id: _, ...rest }) => rest);
    const stringParams = qs.stringify(selectedFilters, { allowDots:true });
    searchParams.set("filters", stringParams);
    setSearchParams(searchParams);
  }, [filters]);

  useEffect(() => {
    searchParams.set("operator", operator);
    setSearchParams(searchParams);
  }, [operator]);

  useEffect(() => {
    if (filtersFromParams.length) {
      setFilters(filtersFromParams);
    }
  }, []);

  return (
    <div className="vm-explore-logs-filter">
      <div className="vm-explore-logs-filter-list">
        {filters.map(({ id, ...rest }) => (
          <FilterBuilderLogs
            {...rest}
            key={id}
            id={id}
            streamsValues={uniqStreamValues}
            extraValues={uniqExtraValues}
            onRemove={handleRemoveFilter}
            onChange={handleChangeFilter}
          />
        ))}
      </div>
      <div className="vm-explore-logs-filter-controls">
        <div className="vm-explore-logs-filter-controls__operator">
          Operator:
          <Toggle
            options={Object.values(FilterOperator).map(o => ({ value: o, title: o }))}
            value={operator}
            onChange={(val) => setOperator(val as FilterOperator)}
          />
        </div>
        <Button
          variant="text"
          color={"error"}
          startIcon={<FilterClearIcon/>}
          onClick={handleClearFilters}
        >
          Clear filters
        </Button>
        <Button
          variant="text"
          startIcon={<PlusIcon/>}
          onClick={handleAddFilter}
        >
          Add filter
        </Button>
      </div>
    </div>
  );
};

export default FilterLogs;
