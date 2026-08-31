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
  sources: string[];
  allSources: string[];
  search: string;
  onChangeKinds: (input: string) => void;
  onChangeSource: (input: string) => void;
  onChangeSearch: (input: string) => void;
}

const NotifiersHeader: FC<NotifiersHeaderProps> = ({
  kinds,
  allKinds,
  sources,
  allSources,
  search,
  onChangeKinds,
  onChangeSource,
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
        {allSources.length > 1 && (
          <div className="vm-explore-alerts-header__vmalert_source">
            <Select
              value={sources}
              list={allSources}
              label="Source"
              placeholder="Please select vmalert source"
              onChange={onChangeSource}
              includeAll
              searchable
              closeOnSelect
            />
          </div>
        )}
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
