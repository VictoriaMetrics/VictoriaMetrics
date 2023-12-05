import React, { FC } from "react";
import { InfoIcon, PlayIcon, WikiIcon } from "../../../components/Main/Icons";
import "./style.scss";
import classNames from "classnames";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import Button from "../../../components/Main/Button/Button";
import QueryEditor from "../../../components/Configurators/QueryEditor/QueryEditor";

export interface ExploreLogHeaderProps {
  query: string;
  error?: string;
  onChange: (val: string) => void;
  onRun: () => void;
}

const ExploreLogsHeader: FC<ExploreLogHeaderProps> = ({ query, error, onChange, onRun }) => {
  const { isMobile } = useDeviceDetect();

  return (
    <div
      className={classNames({
        "vm-explore-logs-header": true,
        "vm-block": true,
        "vm-block_mobile": isMobile,
      })}
    >
      <div className="vm-explore-logs-header__input">
        <QueryEditor
          value={query}
          autocomplete={false}
          onArrowUp={() => null}
          onArrowDown={() => null}
          onEnter={onRun}
          onChange={onChange}
          label={"Log query"}
          error={error}
        />
      </div>
      <div className="vm-explore-logs-header-bottom">
        <div className="vm-explore-logs-header-bottom-helpful">
          <a
            className="vm-link vm-link_with-icon"
            target="_blank"
            href="https://docs.victoriametrics.com/VictoriaLogs/LogsQL.html"
            rel="help noreferrer"
          >
            <InfoIcon/>
            Query language docs
          </a>
          <a
            className="vm-link vm-link_with-icon"
            target="_blank"
            href="https://docs.victoriametrics.com/VictoriaLogs/"
            rel="help noreferrer"
          >
            <WikiIcon/>
            Documentation
          </a>
        </div>
        <div className="vm-explore-logs-header-bottom__execute">
          <Button
            startIcon={<PlayIcon/>}
            onClick={onRun}
            fullWidth
          >
           Execute Query
          </Button>
        </div>
      </div>
    </div>
  );
};

export default ExploreLogsHeader;
