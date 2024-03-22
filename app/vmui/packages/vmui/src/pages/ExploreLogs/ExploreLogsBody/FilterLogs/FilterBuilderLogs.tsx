import React, { FC, useEffect, useMemo, useState } from "preact/compat";
import Select from "../../../../components/Main/Select/Select";
import "./style.scss";
import Button from "../../../../components/Main/Button/Button";
import { CloseIcon } from "../../../../components/Main/Icons";
import Tooltip from "../../../../components/Main/Tooltip/Tooltip";
import { FilterObj, FilterType } from "./FilterLogs";

interface Props extends FilterObj {
  streamsValues: Record<string, string[]>;
  extraValues: Record<string, string[]>;
  onChange: (filter: FilterObj) => void;
  onRemove: (id: number) => void;
}

const FilterBuilderLogs: FC<Props> = ({
  id,
  type: typeProp,
  field: fieldProp,
  value: valueProp,
  streamsValues,
  extraValues,
  onRemove,
  onChange
}) => {
  const [filter, setFilter] = useState<FilterObj>({
    id,
    type: typeProp || FilterType.stream,
    field: fieldProp,
    value: valueProp
  });

  const keyList = useMemo(() => {
    switch (filter.type) {
      case FilterType.stream:
        return Object.keys(streamsValues);
      case FilterType.extra:
        return Object.keys(extraValues);
      default:
        return [];
    }
  }, [filter, streamsValues, extraValues]);

  const valueList = useMemo(() => {
    switch (filter.type) {
      case FilterType.stream:
        return Object.values(streamsValues[filter.field] || []);
      case FilterType.extra:
        return Object.values(extraValues[filter.field] || []);
      default:
        return [];
    }
  }, [filter, streamsValues, extraValues]);

  const handleChangeType = (val: string) => {
    setFilter(prev => ({
      ...prev,
      type: val as FilterType,
      field: "",
      value: ""
    }));
  };

  const handleChangeField = (val: string) => {
    setFilter(prev => ({
      ...prev,
      field: val,
      value: ""
    }));
  };

  const handleChangeValue = (val: string) => {
    setFilter(prev => ({
      ...prev,
      value: val
    }));
  };

  useEffect(() => {
    onChange(filter);
  }, [filter]);

  return (
    <div className="vm-explore-logs-filter-builder">
      <div className="vm-explore-logs-filter-builder__item">
        <Select
          value={filter.type}
          list={Object.values(FilterType)}
          label={"Field"}
          placeholder={"Please select field"}
          onChange={handleChangeType}
        />
      </div>
      <div className="vm-explore-logs-filter-builder__item">
        <Select
          searchable
          value={filter.field}
          list={keyList}
          label={"Key"}
          placeholder={"Please select key"}
          onChange={handleChangeField}
        />
      </div>
      <div className="vm-explore-logs-filter-builder__item vm-explore-logs-filter-builder__item_values">
        <Select
          searchable
          value={filter.value}
          list={valueList}
          label={"Value"}
          placeholder={"Please select value"}
          onChange={handleChangeValue}
        />
      </div>
      <Tooltip title={"Remove"}>
        <Button
          variant="text"
          color={"gray"}
          startIcon={<CloseIcon/>}
          onClick={() => onRemove(id)}
          ariaLabel={"filters"}
        />
      </Tooltip>
    </div>
  );
};

export default FilterBuilderLogs;
