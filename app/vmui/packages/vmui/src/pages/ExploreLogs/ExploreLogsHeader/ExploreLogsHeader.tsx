import { FC, useEffect, useState } from "preact/compat";
import { InfoIcon, PlayIcon, SpinnerIcon, WikiIcon } from "../../../components/Main/Icons";
import "./style.scss";
import classNames from "classnames";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import Button from "../../../components/Main/Button/Button";
import QueryEditor from "../../../components/Configurators/QueryEditor/QueryEditor";
import TextField from "../../../components/Main/TextField/TextField";
import LogsQueryEditorAutocomplete from "../../../components/Configurators/QueryEditor/LogsQL/LogsQueryEditorAutocomplete";
import { useQueryDispatch, useQueryState } from "../../../state/query/QueryStateContext";
import Switch from "../../../components/Main/Switch/Switch";
import QueryHistory from "../../../components/QueryHistory/QueryHistory";
import useBoolean from "../../../hooks/useBoolean";
import { useQuickAutocomplete } from "../../../hooks/useQuickAutocomplete";
import { AUTOCOMPLETE_QUICK_KEY } from "../../../components/Main/ShortcutKeys/constants/keyList";
import Tooltip from "../../../components/Main/Tooltip/Tooltip";

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
  const { autocomplete, queryHistory, autocompleteQuick } = useQueryState();
  const queryDispatch = useQueryDispatch();
  const setQuickAutocomplete = useQuickAutocomplete();

  const [errorLimit, setErrorLimit] = useState("");
  const [limitInput, setLimitInput] = useState(limit);
  const { value: awaitQuery, setValue: setAwaitQuery } = useBoolean(false);

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

  const onChangeAutocomplete = () => {
    queryDispatch({ type: "TOGGLE_AUTOCOMPLETE" });
  };

  useEffect(() => {
    setLimitInput(limit);
  }, [limit]);

  const handleHistoryChange = (step: number) => {
    const { values, index } = queryHistory[0];
    const newIndexHistory = index + step;
    if (newIndexHistory < 0 || newIndexHistory >= values.length) return;
    onChange(values[newIndexHistory] || "");
    queryDispatch({
      type: "SET_QUERY_HISTORY_BY_INDEX",
      payload: { value: { values, index: newIndexHistory }, queryNumber: 0 }
    });
  };

  const handleSelectHistory = (value: string) => {
    onChange(value);
    setAwaitQuery(true);
  };

  const createHandlerArrow = (step: number) => () => {
    handleHistoryChange(step);
  };

  useEffect(() => {
    if (awaitQuery) {
      onRun();
      setAwaitQuery(false);
    }
  }, [query, awaitQuery]);

  const onChangeHandle = (value: string) => {
    onChange(value);
    if (autocompleteQuick) {
      setQuickAutocomplete(false);
    }
  };

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
          autocomplete={autocomplete || autocompleteQuick}
          autocompleteEl={LogsQueryEditorAutocomplete}
          onArrowUp={createHandlerArrow(-1)}
          onArrowDown={createHandlerArrow(1)}
          onEnter={onRun}
          onChange={onChangeHandle}
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
        <div className="vm-explore-logs-header-bottom-contols">
          <Tooltip title={<>Quick tip: {AUTOCOMPLETE_QUICK_KEY}</>}>
            <Switch
              label={"Autocomplete"}
              value={autocomplete}
              onChange={onChangeAutocomplete}
              fullWidth={isMobile}
            />
          </Tooltip>
        </div>
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
        <div className="vm-explore-logs-header-bottom-buttons">
          <QueryHistory
            handleSelectQuery={handleSelectHistory}
            historyKey={"LOGS_QUERY_HISTORY"}
          />
          <div className="vm-explore-logs-header-bottom-execute">
            <Button
              startIcon={isLoading ? <SpinnerIcon/> : <PlayIcon/>}
              onClick={onRun}
              fullWidth
            >
              <div>
                <span className="vm-explore-logs-header-bottom-execute__text">
                  {isLoading ? "Cancel Query" : "Execute Query"}
                </span>
                <span className="vm-explore-logs-header-bottom-execute__text_hidden">Execute Query</span>
              </div>
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
};

export default ExploreLogsHeader;
