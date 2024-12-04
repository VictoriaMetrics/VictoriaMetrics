import React, { FC, useEffect, useState } from "preact/compat";
import { InfoIcon, PlayIcon, SpinnerIcon, WikiIcon } from "../../../components/Main/Icons";
import "./style.scss";
import classNames from "classnames";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import Button from "../../../components/Main/Button/Button";
import QueryEditor from "../../../components/Configurators/QueryEditor/QueryEditor";
import TextField from "../../../components/Main/TextField/TextField";

export interface ExploreLogHeaderProps {
  query: string;
  limit: number;
  error?: string;
  isLoading: boolean;
  onChange: (val: string) => void;
  onChangeLimit: (val: number) => void;
  onRun: () => void;
}

const ExploreLogsHeader: FC<ExploreLogHeaderProps> = ({
  query,
  limit,
  error,
  isLoading,
  onChange,
  onChangeLimit,
  onRun,
}) => {
  const { isMobile } = useDeviceDetect();

  const [errorLimit, setErrorLimit] = useState("");
  const [limitInput, setLimitInput] = useState(limit);

  const handleChangeLimit = (val: string) => {
    const number = +val;
    setLimitInput(number);
    if (isNaN(number) || number < 0) {
      setErrorLimit("Number must be bigger than zero");
    } else {
      setErrorLimit("");
      onChangeLimit(number);
    }
  };

  useEffect(() => {
    setLimitInput(limit);
  }, [limit]);

  return (
    <div
      className={classNames({
        "vm-explore-logs-header": true,
        "vm-block": true,
        "vm-block_mobile": isMobile,
      })}
    >
      <div className="vm-explore-logs-header-top">
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
        <TextField
          label="Limit entries"
          type="number"
          value={limitInput}
          error={errorLimit}
          onChange={handleChangeLimit}
          onEnter={onRun}
        />
      </div>
      <div className="vm-explore-logs-header-bottom">
        <div className="vm-explore-logs-header-bottom-contols"></div>
        <div className="vm-explore-logs-header-bottom-helpful">
          <a
            className="vm-link vm-link_with-icon"
            target="_blank"
            href="https://docs.victoriametrics.com/victorialogs/logsql/"
            rel="help noreferrer"
          >
            <InfoIcon/>
            Query language docs
          </a>
          <a
            className="vm-link vm-link_with-icon"
            target="_blank"
            href="https://docs.victoriametrics.com/victorialogs/"
            rel="help noreferrer"
          >
            <WikiIcon/>
            Documentation
          </a>
        </div>
        <div className="vm-explore-logs-header-bottom-execute">
          <Button
            startIcon={isLoading ? <SpinnerIcon/> : <PlayIcon/>}
            onClick={onRun}
            fullWidth
          >
            <span className="vm-explore-logs-header-bottom-execute__text">
              {isLoading ? "Cancel Query" : "Execute Query"}
            </span>
            <span className="vm-explore-logs-header-bottom-execute__text_hidden">Execute Query</span>
          </Button>
        </div>
      </div>
    </div>
  );
};

export default ExploreLogsHeader;
