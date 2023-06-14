import React, { FC, useEffect } from "preact/compat";
import useDeviceDetect from "../../hooks/useDeviceDetect";
import classNames from "classnames";
import "./style.scss";
import ExploreLogBody from "./ExploreLogPanel/ExploreLogBody";
import TextField from "../../components/Main/TextField/TextField";
import { PlayIcon } from "../../components/Main/Icons";
import Button from "../../components/Main/Button/Button";
import useStateSearchParams from "../../hooks/useStateSearchParams";
import useSearchParamsFromObject from "../../hooks/useSearchParamsFromObject";
import { useFetchLogs } from "./hooks/useFetchLogs";
import { useAppState } from "../../state/common/StateContext";
import Spinner from "../../components/Main/Spinner/Spinner";
import Alert from "../../components/Main/Alert/Alert";

const ExploreLog: FC = () => {
  const { isMobile } = useDeviceDetect();
  const { serverUrl } = useAppState();
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();

  const [query, setQuery] = useStateSearchParams("", "query");
  const { logs, isLoading, error, fetchLogs } = useFetchLogs(serverUrl, query);

  const handleRunQuery = () => {
    fetchLogs();
    setSearchParamsFromKeys({ query });
  };

  useEffect(() => {
    if (query) handleRunQuery();
  }, []);

  return (
    <div className="vm-explore-log">
      <div
        className={classNames({
          "vm-explore-log-header": true,
          "vm-block": true,
          "vm-block_mobile": isMobile,
        })}
      >
        <TextField
          autofocus
          label="Log query"
          value={query}
          onChange={setQuery}
          onEnter={handleRunQuery}
          inputmode="url"
        />
        <Button
          startIcon={<PlayIcon/>}
          onClick={handleRunQuery}
          fullWidth
        >
          Execute Query
        </Button>
      </div>
      {isLoading && <Spinner />}
      {error && <Alert variant="error">{error}</Alert>}
      <ExploreLogBody data={logs}/>
    </div>
  );
};

export default ExploreLog;
