import React, { FC, useCallback, useEffect, useMemo, useRef, useState } from "preact/compat";
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
import Popper from "../../../components/Main/Popper/Popper";
import helperText from "./helperText";

type Props = {
  fetchUrl?: string[];
}

const getDefaultReportName = () => `vmui_report_${dayjs().utc().format(DATE_FILENAME_FORMAT)}`;

const DownloadReport: FC<Props> = ({ fetchUrl }) => {
  const { query } = useQueryState();

  const [filename, setFilename] = useState(getDefaultReportName());
  const [comment, setComment] = useState("");
  const [trace, setTrace] = useState(true);
  const [error, setError] = useState<ErrorTypes | string>();
  const [isLoading, setIsLoading] = useState(false);

  const filenameRef = useRef<HTMLDivElement>(null);
  const commentRef = useRef<HTMLDivElement>(null);
  const traceRef = useRef<HTMLDivElement>(null);
  const generateRef = useRef<HTMLDivElement>(null);
  const helperRefs = [filenameRef, commentRef, traceRef, generateRef];
  const [stepHelper, setStepHelper] = useState(0);

  const {
    value: openModal,
    toggle: toggleOpen,
    setFalse: handleClose,
  } = useBoolean(false);

  const {
    value: openHelper,
    toggle: toggleHelper,
    setFalse: handleCloseHelper,
  } = useBoolean(false);

  const fetchUrlReport = useMemo(() => {
    if (!fetchUrl) return;
    return fetchUrl.map((str, i) => {
      const url = new URL(str);
      trace ? url.searchParams.set("trace", "1") : url.searchParams.delete("trace");
      return { id: i, url: url };
    });
  }, [fetchUrl, trace]);

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

  const handleChangeHelp = (step: number) => () => {
    setStepHelper(prevStep => prevStep + step);
  };

  useEffect(() => {
    setError("");
    setFilename(getDefaultReportName());
    setComment("");
  }, [openModal]);

  useEffect(() => {
    setStepHelper(0);
  }, [openHelper]);

  return (
    <>
      <Tooltip title={"Export query"}>
        <Button
          variant="text"
          startIcon={<DownloadIcon/>}
          onClick={toggleOpen}
          ariaLabel="export query"
        />
      </Tooltip>
      {openModal && (
        <Modal
          title={"Export query"}
          onClose={handleClose}
          isOpen={openModal}
        >
          <div className="vm-download-report">
            <div className="vm-download-report-settings">
              <div ref={filenameRef}>
                <TextField
                  label="Filename"
                  value={filename}
                  onChange={setFilename}
                />
              </div>
              <div ref={commentRef}>
                <TextField
                  type="textarea"
                  label="Comment"
                  value={comment}
                  onChange={setComment}
                />
              </div>
              <div ref={traceRef}>
                <Checkbox
                  checked={trace}
                  onChange={setTrace}
                  label={"Include query trace"}
                />
              </div>
            </div>
            {error && <Alert variant="error">{error}</Alert>}
            <div className="vm-download-report__buttons">
              <Button
                variant="text"
                onClick={toggleHelper}
              >
                Help
              </Button>
              <div ref={generateRef}>
                <Button
                  onClick={handleGenerateReport}
                  disabled={isLoading}
                >
                  {isLoading ? "Loading data..." : "Generate Report"}
                </Button>
              </div>
            </div>
            <Popper
              open={openHelper}
              buttonRef={helperRefs[stepHelper]}
              placement="top-left"
              variant="dark"
              onClose={handleCloseHelper}
            >
              <div className="vm-download-report-helper">
                <div className="vm-download-report-helper__description">
                  {helperText[stepHelper]}
                </div>
                <div className="vm-download-report-helper__buttons">
                  {stepHelper !== 0 && (
                    <Button
                      onClick={handleChangeHelp(-1)}
                      size="small"
                      color={"white"}
                    >
                      Prev
                    </Button>
                  )}
                  <Button
                    onClick={stepHelper === helperRefs.length - 1 ? handleCloseHelper : handleChangeHelp(1)}
                    size="small"
                    color={"white"}
                    variant={"text"}
                  >
                    {stepHelper === helperRefs.length - 1 ? "Close" : "Next"}
                  </Button>
                </div>
              </div>
            </Popper>
          </div>
        </Modal>
      )}
    </>
  );
};

export default DownloadReport;
