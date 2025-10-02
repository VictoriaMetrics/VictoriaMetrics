import { FC, useCallback, useEffect, useRef, useState } from "preact/compat";
import { DebugIcon } from "../../../components/Main/Icons";
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
import Popper from "../../../components/Main/Popper/Popper";
import helperText from "./helperText";
import { Link } from "react-router-dom";
import router from "../../../router";
import { parseLineToJSON } from "../../../utils/json";
import { ExportMetricResult, ReportMetaData } from "../../../api/types";
import { getApiEndpoint } from "../../../utils/url";
import MarkdownEditor from "../../../components/Main/MarkdownEditor/MarkdownEditor";
import { downloadJSON } from "../../../utils/file";

export enum ReportType {
  QUERY_DATA,
  RAW_DATA,
}

type Props = {
  fetchUrl?: string[];
  reportType?: ReportType
}

type MetaData = {
  id: number;
  url: URL;
  title: string;
  comment: string;
}

const getDefaultTitle = (type: ReportType) => {
  switch (type) {
    case ReportType.RAW_DATA:
      return "Raw report";
    default:
      return "Report";
  }
};

const getDefaultFilename = (title: string) => {
  const timestamp = dayjs().utc().format(DATE_FILENAME_FORMAT);
  return `vmui_${title.toLowerCase().replace(/ /g, "_")}_${timestamp}`;
};

const DownloadReport: FC<Props> = ({ fetchUrl, reportType = ReportType.QUERY_DATA }) => {
  const { query } = useQueryState();

  const defaultTitle = getDefaultTitle(reportType);
  const defaultFilename = getDefaultFilename(defaultTitle);

  const [title, setTitle] = useState(defaultTitle);
  const [filename, setFilename] = useState(defaultFilename);
  const [comment, setComment] = useState("");
  const [trace, setTrace] = useState(reportType === ReportType.QUERY_DATA);
  const [error, setError] = useState<ErrorTypes | string>();
  const [isLoading, setIsLoading] = useState(false);

  const titleRef = useRef<HTMLDivElement>(null);
  const filenameRef = useRef<HTMLDivElement>(null);
  const commentRef = useRef<HTMLDivElement>(null);
  const traceRef = useRef<HTMLDivElement>(null);
  const generateRef = useRef<HTMLDivElement>(null);
  const helperRefs = [filenameRef, titleRef, commentRef, traceRef, generateRef];
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

  const getFetchUrlReport = useCallback(() => {
    if (!fetchUrl) return;
    try {
      return fetchUrl.map((str, i) => {
        const url = new URL(str);
        trace ? url.searchParams.set("trace", "1") : url.searchParams.delete("trace");
        return { id: i, url: url };
      });
    } catch (e) {
      setError(String(e));
    }
  }, [fetchUrl, trace]);

  const generateFile = useCallback((data: unknown) => {
    const json = JSON.stringify(data, null, 2);
    downloadJSON(json, `${filename || defaultFilename}.json`);
    handleClose();
  }, [filename]);

  const getMetaData = ({ id, url, comment, title }: MetaData): ReportMetaData => {
    return {
      id,
      title: title || defaultTitle,
      comment,
      endpoint: getApiEndpoint(url.pathname) || "",
      params: Object.fromEntries(url.searchParams)
    };
  };

  const processJsonLineResponse = async (response: Response, metaData: MetaData) => {
    const result: { metric: { [p: string]: string }, values: number[][] }[] = [];
    const text = await response.text();

    if (response.ok) {
      const lines = text.split("\n").filter(line => line);
      lines.forEach((line: string) => {
        const jsonLine = parseLineToJSON(line) as (ExportMetricResult | null);
        if (!jsonLine) return;
        result.push({
          metric: jsonLine.metric,
          values: jsonLine.values.map((value, index) => [(jsonLine.timestamps[index] / 1000), value]),
        });
      });
    } else {
      setError(String(text));
    }

    return { data: { result, resultType: "matrix" }, vmui: getMetaData(metaData) };
  };

  const processJsonResponse = async (response: Response, metaData: MetaData) => {
    const resp = await response.json();

    if (response.ok) {
      resp.vmui = getMetaData(metaData);
      return resp;
    } else {
      const errorType = resp.errorType ? `${resp.errorType}\r\n` : "";
      setError(`${errorType}${resp?.error || resp?.message || "unknown error"}`);
    }
  };

  const processResponse = async (response: Response, metaData: MetaData) => {
    switch (reportType) {
      case ReportType.RAW_DATA:
        return await processJsonLineResponse(response, metaData);
      default:
        return await processJsonResponse(response, metaData);
    }
  };

  const handleGenerateReport = useCallback(async () => {
    const fetchUrlReport = getFetchUrlReport();

    if (!fetchUrlReport) {
      setError(prev => !prev ? ErrorTypes.validQuery : prev);
      return;
    }

    setError("");
    setIsLoading(true);

    try {
      const result = [];
      for await (const fetchOps of fetchUrlReport) {
        if (!fetchOps) continue;
        const { url, id } = fetchOps;
        const response = await fetch(url);
        const data = await processResponse(response, { id, url, comment, title });
        result.push(data);
      }
      result.length && generateFile(result);
    } catch (e) {
      if (e instanceof Error && e.name !== "AbortError") {
        setError(`${e.name}: ${e.message}`);
      }
    } finally {
      setIsLoading(false);
    }
  }, [getFetchUrlReport, comment, generateFile, query, title]);

  const handleChangeHelp = (step: number) => () => {
    const findNextRef = (index: number): number => {
      const nextIndex = index + step;
      if (helperRefs[nextIndex]?.current) return nextIndex;
      return findNextRef(nextIndex);
    };
    setStepHelper(findNextRef);
  };

  useEffect(() => {
    setError("");
    setFilename(defaultFilename);
    setComment("");
  }, [openModal]);

  useEffect(() => {
    setStepHelper(0);
  }, [openHelper]);

  const RawQueryLink = () => (
    <Link
      className="vm-link vm-link_underlined vm-link_colored"
      to={router.rawQuery}
    >
      Raw Query
    </Link>
  );

  return (
    <>
      <Tooltip title={"Debug query"}>
        <Button
          variant="text"
          startIcon={<DebugIcon />}
          onClick={toggleOpen}
          ariaLabel="Debug query"
        />
      </Tooltip>
      {openModal && (
        <Modal
          title={"Debug query"}
          onClose={handleClose}
          isOpen={openModal}
        >
          <div className="vm-download-report">
            <div className="vm-download-report-settings">
              <div ref={filenameRef}>
                <div className="vm-download-report-settings__title">Filename</div>
                <TextField
                  value={filename}
                  onChange={setFilename}
                />
              </div>
              <div ref={titleRef}>
                <div className="vm-download-report-settings__title">Report title</div>
                <TextField
                  value={title}
                  onChange={setTitle}
                />
              </div>
              <div ref={commentRef}>
                <div className="vm-download-report-settings__title">Comment</div>
                <MarkdownEditor
                  value={comment}
                  onChange={setComment}
                />
              </div>
              {reportType === ReportType.QUERY_DATA && (
                <>
                  <div ref={traceRef}>
                    <Checkbox
                      checked={trace}
                      onChange={setTrace}
                      label={"Include query trace"}
                    />
                  </div>
                  <Alert variant="info">
                    If confused with the query results,
                    try viewing the raw samples for selected series in <RawQueryLink/> tab.
                  </Alert>
                </>
              )}
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
