import { FC, useMemo, useRef } from "preact/compat";
import Button from "../../../../../../components/Main/Button/Button";
import { SettingsIcon, SortArrowDownIcon, SortArrowUpIcon, SortIcon } from "../../../../../../components/Main/Icons";
import Tooltip from "../../../../../../components/Main/Tooltip/Tooltip";
import Select from "../../../../../../components/Main/Select/Select";
import useBoolean from "../../../../../../hooks/useBoolean";
import { useState, useEffect, useCallback } from "react";
import Modal from "../../../../../../components/Main/Modal/Modal";
import { useSearchParams } from "react-router-dom";
import "./style.scss";
import { SortDirection } from "../types";

const title = "JSON settings";
const directionList = ["asc", "desc"];

interface JsonSettingsProps {
  fields: string[];
  sortQueryParamName: string;
  fieldSortQueryParamName: string;
}

const JsonViewSettings: FC<JsonSettingsProps> = ({
  fields,
  sortQueryParamName,
  fieldSortQueryParamName
}) => {
  const [searchParams, setSearchParams] = useSearchParams();
  const buttonRef = useRef<HTMLDivElement>(null);
  const [fieldSortDirection, setFieldSortDirection] = useState<SortDirection>(null);

  const {
    value: openSettings,
    toggle: toggleOpenSettings,
    setFalse: handleClose,
  } = useBoolean(false);
  
  const [sortField, setSortField] = useState<string | null>(null);
  const [sortDirection, setSortDirection] = useState<SortDirection>(null);

  useEffect(() => {
    const sortParam = searchParams.get(sortQueryParamName);
    const isSortDirection = (value: string) : value is Exclude<SortDirection, null> => directionList.includes(value);
    if (sortParam) {
      const [field, direction] = sortParam.split(":").map(decodeURIComponent);
      if (field && (isSortDirection(direction))) {
        setSortField(field);
        setSortDirection(direction);
      }
    }

    const fieldSortParam = searchParams.get(fieldSortQueryParamName);
    if (fieldSortParam === "asc" || fieldSortParam === "desc") {
      setFieldSortDirection(fieldSortParam);
    }
  }, [searchParams, sortQueryParamName, fieldSortQueryParamName, setSortField, setSortDirection, setFieldSortDirection]);

  const updateSortParams = useCallback((field: string | null, direction: SortDirection) => {
    const updatedParams = new URLSearchParams(searchParams.toString());

    if (!field || !direction) {
      updatedParams.delete(sortQueryParamName);
    } else {
      updatedParams.set(sortQueryParamName, `${field}:${direction || ""}`);
    }

    setSearchParams(updatedParams);
  }, [searchParams, sortQueryParamName]);

  const handleSort = (field: string) => {
    const newDirection: SortDirection = sortDirection || "asc";
    setSortField(field);
    setSortDirection(newDirection);
    updateSortParams(field, newDirection);
  };

  const resetSort = () => {
    setSortField(null);
    setSortDirection(null);
    updateSortParams(null, null);
  };

  const changeFieldSortDirection = useCallback(() => {
    let newFieldSortDirection: SortDirection = null;
    if (fieldSortDirection === null) {
      newFieldSortDirection = "asc";
    }else if (fieldSortDirection === "asc") {
      newFieldSortDirection = "desc";
    }
    setFieldSortDirection(newFieldSortDirection);
    const updatedParams = new URLSearchParams(searchParams.toString());

    if (!newFieldSortDirection) {
      updatedParams.delete(fieldSortQueryParamName);
    } else {
      updatedParams.set(fieldSortQueryParamName, encodeURIComponent(newFieldSortDirection));
    }

    setSearchParams(updatedParams);
  },[fieldSortDirection, searchParams, fieldSortQueryParamName]);

  const handleChangeSortDirection = (direction: string) => {
    const field = sortField || fields[0];
    setSortField(field);
    setSortDirection(direction as SortDirection);
    updateSortParams(field, direction as SortDirection);
  };

  const fieldSortMeta = useMemo(() => ({
    default: {
      title: "Set field sort order. Click to sort in ascending order",
      icon: <SortIcon />
    },
    asc: {
      title: "Fields sorted ascending. Click to sort in descending order",
      icon: <SortArrowDownIcon />
    },
    desc: {
      title: "Fields sorted descending. Click to reset sort",
      icon: <SortArrowUpIcon />
    },
  }), []);

  const fieldSortButton = useMemo(() => {
    const { title, icon } = fieldSortMeta[fieldSortDirection ?? "default"];
    return <Tooltip title={title}>
      <Button
        variant="text"
        startIcon={icon}
        onClick={changeFieldSortDirection}
        ariaLabel={title}
      />
    </Tooltip>;
  }, [fieldSortDirection, toggleOpenSettings, changeFieldSortDirection, fieldSortMeta]);


  return (
    <div className="vm-json-settings">
      {fieldSortButton}
      <Tooltip title={title}>
        <div ref={buttonRef}>
          <Button
            variant="text"
            startIcon={<SettingsIcon/>}
            onClick={toggleOpenSettings}
            ariaLabel={title}
          />
        </div>
      </Tooltip>
      {openSettings && (
        <Modal
          title={title}
          className="vm-json-settings-modal"
          onClose={handleClose}
        >
          <div className="vm-json-settings-modal-section">
            <div className="vm-json-settings-modal-section__sort-settings-container">
              <Select
                value={sortField || ""}
                onChange={handleSort}
                list={fields}
                label="Select field"
              />
              <Select
                value={sortDirection || ""}
                onChange={handleChangeSortDirection}
                list={directionList}
                label="Sort direction"
              />
              {(sortField || sortDirection) && (
                <Button
                  variant="outlined"
                  color="error"
                  onClick={resetSort}
                >
                  Reset sort
                </Button>
              )}
            </div>
          </div>
        </Modal>)}
    </div>
  );
};

export default JsonViewSettings;
