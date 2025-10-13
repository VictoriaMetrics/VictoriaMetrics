import { FC } from "preact/compat";
import Select from "../../Main/Select/Select";
import { SearchIcon } from "../../Main/Icons";
import TextField from "../../Main/TextField/TextField";
import "./style.scss";
import classNames from "classnames";
import useDeviceDetect from "../../../hooks/useDeviceDetect";

interface NotifiersHeaderProps {
  kinds: string[];
  allKinds: string[];
  search: string;
  onChangeKinds: (input: string) => void;
  onChangeSearch: (input: string) => void;
}

const NotifiersHeader: FC<NotifiersHeaderProps> = ({
  kinds,
  allKinds,
  search,
  onChangeKinds,
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
            value={kinds}
            list={allKinds}
            label="Notifier type"
            placeholder="Please select notifier type"
            onChange={onChangeKinds}
            autofocus={!!kinds.length && !isMobile}
            includeAll
            searchable
          />
        </div>
        <div className="vm-explore-alerts-header-search">
          <TextField
            label="Search"
            value={search}
            placeholder="Filter by kind, address or labels"
            startIcon={<SearchIcon />}
            onChange={onChangeSearch}
          />
        </div>
      </div>
    </>
  );
};

export default NotifiersHeader;
