import React, { FC } from "preact/compat";
import Select from "../../Main/Select/Select";
import { SearchIcon } from "../../Main/Icons";
import TextField from "../../Main/TextField/TextField";
import "./style.scss";
import classNames from "classnames";
import useDeviceDetect from "../../../hooks/useDeviceDetect";

interface NotifiersHeaderProps {
  types: string[]
  allTypes: string[]
  onChangeTypes: (types: string) => void
  onChangeSearch: (input: string) => void
}

const NotifiersHeader: FC<NotifiersHeaderProps> = ({
  types,
  allTypes,
  onChangeTypes,
  onChangeSearch,
}) => {
  const { isMobile } = useDeviceDetect();

  return (
    <>
      <div
        className={classNames({
          "vm-explore-alerts-header": true,
          "vm-explore-alerts-header_mobile": isMobile,
          "vm-block": true,
          "vm-block_mobile": isMobile,
        })}
      >
        <div className="vm-explore-alerts-header__rule_type">
          <Select
            value={types}
            list={allTypes}
            label="Notifier type"
            placeholder="Please select notifier type"
            onChange={onChangeTypes}
            autofocus={!types && !!types.length && !isMobile}
            includeAll
            searchable
          />
        </div>
        <div className="vm-explore-alerts-header-search">
          <TextField
            label="Search"
            placeholder="Fitler by kind, address or labels"
            startIcon={<SearchIcon/>}
            onChange={onChangeSearch}
          />
        </div>
      </div>
    </>
  );
};

export default NotifiersHeader;
