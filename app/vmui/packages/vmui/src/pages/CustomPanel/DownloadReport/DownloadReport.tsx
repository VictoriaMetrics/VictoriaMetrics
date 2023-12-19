import React, { FC, useCallback, useEffect, useMemo, useState } from "preact/compat";
import { DownloadIcon } from "../../../components/Main/Icons";
import Button from "../../../components/Main/Button/Button";
import Tooltip from "../../../components/Main/Tooltip/Tooltip";
import useBoolean from "../../../hooks/useBoolean";
import "./style.scss";
import Checkbox from "../../../components/Main/Checkbox/Checkbox";
import Modal from "../../../components/Main/Modal/Modal";
import dayjs from "dayjs";
import { DATE_FILENAME_FORMAT } from "../../../constants/date";
import TextField from "../../../components/Main/TextField/TextField";
import { useQueryState } from "../../../state/query/QueryStateContext";
import { ErrorTypes } from "../../../types";
import Alert from "../../../components/Main/Alert/Alert";
import qs from "qs";

type Props = {
  fetchUrl?: string[];
}

enum SettingsReport {
  tracing = "trace query",
}

const getDefaultReportName = () => `vmui_report_${dayjs().utc().format(DATE_FILENAME_FORMAT)}`;

const DownloadReport: FC<Props> = ({ fetchUrl }) => {
  const { query } = useQueryState();

  const [filename, setFilename] = useState(getDefaultReportName());
  const [comment, setComment] = useState("");
  const [error, setError] = useState<ErrorTypes | string>();
  const [isLoading, setIsLoading] = useState(false);

  const [settings, setSettings] = useState({
    [SettingsReport.tracing]: true,
  });

  const {
    value: openModal,
    toggle: toggleOpen,
    setFalse: handleClose,
  } = useBoolean(false);

  const fetchUrlReport = useMemo(() => {
    if (!fetchUrl) return;
    return fetchUrl.map((str, i) => {
      const url = new URL(str);
      settings[SettingsReport.tracing] ? url.searchParams.set("trace", "1") : url.searchParams.delete("trace");
      return { id: i, url: url };
    });
  }, [fetchUrl]);

  const handlerChangeSetting = (key: string) => (value: boolean) => {
    setSettings(prev => ({ ...prev, [key]: value }));
  };

  const generateFile = useCallback((data: unknown) => {
    const json = JSON.stringify(data, null, 2);
    const blob = new Blob([json], { type: "application/json" });
    const href = URL.createObjectURL(blob);

    const link = document.createElement("a");
    link.href = href;
    link.download = `${filename || getDefaultReportName()}.json`;
    document.body.appendChild(link);
    link.click();

    document.body.removeChild(link);
    URL.revokeObjectURL(href);
    handleClose();
  }, [filename]);

  const handleGenerateReport = useCallback(async () => {
    if (!fetchUrlReport) {
      setError(ErrorTypes.validQuery);
      return;
    }

    setError("");
    setIsLoading(true);

    try {
      const result = [];
      for await (const { url, id } of fetchUrlReport) {
        const response = await fetch(url);
        const resp = await response.json();
        if (response.ok) {
          resp.vmui = {
            id,
            comment,
            params: qs.parse(new URL(url).search.replace(/^\?/, ""))
          };
          result.push(resp);
        } else {
          const errorType = resp.errorType ? `${resp.errorType}\r\n` : "";
          setError(`${errorType}${resp?.error || resp?.message || "unknown error"}`);
        }
      }
      result.length && generateFile(result);
    } catch (e) {
      if (e instanceof Error && e.name !== "AbortError") {
        setError(`${e.name}: ${e.message}`);
      }
    } finally {
      setIsLoading(false);
    }
  }, [fetchUrlReport, comment, generateFile, query]);

  useEffect(() => {
    setError("");
    setFilename(getDefaultReportName());
    setComment("");
  }, [openModal]);

  return (
    <>
      <Tooltip title={"Report query"}>
        <Button
          variant="text"
          startIcon={<DownloadIcon/>}
          onClick={toggleOpen}
          ariaLabel="report query"
        />
      </Tooltip>
      {openModal && (
        <Modal
          title={"Report query"}
          onClose={handleClose}
          isOpen={openModal}
        >
          <div className="vm-download-report">
            <div className="vm-download-report-settings">
              <TextField
                label="Filename"
                value={filename}
                onChange={setFilename}
              />
              <TextField
                type="textarea"
                label="Comment"
                value={comment}
                onChange={setComment}
              />
              {Object.entries(settings).map(([key, value]) => (
                <Checkbox
                  key={key}
                  checked={value}
                  onChange={handlerChangeSetting(key)}
                  label={`Include ${key}`}
                />
              ))}
            </div>
            {error && <Alert variant="error">{error}</Alert>}
            <div className="vm-download-report__buttons">
              <Button
                onClick={handleGenerateReport}
                disabled={isLoading}
              >
                {isLoading ? "Loading data..." : "Generate Report"}
              </Button>
            </div>
          </div>
        </Modal>
      )}
    </>
  );
};

export default DownloadReport;
