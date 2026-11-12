import { FC } from "preact/compat";
import Select from "../../Main/Select/Select";
import { RefreshIcon, SearchIcon } from "../../Main/Icons";
import TextField from "../../Main/TextField/TextField";
import "./style.scss";
import classNames from "classnames";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import Button from "../../Main/Button/Button";
import { useTimeDispatch } from "../../../state/time/TimeStateContext";

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
  const dispatch = useTimeDispatch();

  const handleRefresh = () => {
    dispatch({ type: "RUN_QUERY" });
  };

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
          <div>
            <Button
              variant="text"
              onClick={handleRefresh}
              startIcon={<RefreshIcon />}
              ariaLabel="Refresh"
            >
              Refresh
            </Button>
          </div>
        </div>
      </div>
    </>
  );
};

export default NotifiersHeader;
