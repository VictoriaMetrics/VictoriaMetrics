import React, { FC, useEffect, useMemo, useState } from "preact/compat";
import { ChangeEvent } from "react";
import Button from "../../components/Main/Button/Button";
import Alert from "../../components/Main/Alert/Alert";
import { CloseIcon } from "../../components/Main/Icons";
import Modal from "../../components/Main/Modal/Modal";
import useDropzone from "../../hooks/useDropzone";
import useBoolean from "../../hooks/useBoolean";
import UploadJsonButtons from "../../components/UploadJsonButtons/UploadJsonButtons";
import JsonForm from "./JsonForm/JsonForm";
import "../TracePage/style.scss";
import QueryAnalyzerView from "./QueryAnalyzerView/QueryAnalyzerView";
import { InstantMetricResult, MetricResult, TracingData } from "../../api/types";
import QueryAnalyzerInfo from "./QueryAnalyzerInfo/QueryAnalyzerInfo";
import { TimeParams } from "../../types";
import { dateFromSeconds, formatDateToUTC, humanizeSeconds } from "../../utils/time";
import { findMostCommonStep } from "./QueryAnalyzerView/utils";

export type DataAnalyzerType = {
  data: {
    resultType: "vector" | "matrix";
    result: MetricResult[] | InstantMetricResult[]
  };
  stats?: {
    seriesFetched?: string;
    executionTimeMsec?: number
  };
  vmui?: {
    id: number;
    comment: string;
    params: Record<string, string>;
  };
  status: string;
  trace?: TracingData;
  isPartial?: boolean;
}

const QueryAnalyzer: FC = () => {
  const [data, setData] = useState<DataAnalyzerType[]>([]);
  const [error, setError] = useState("");
  const hasData = useMemo(() => !!data.length, [data]);

  const {
    value: openModal,
    setTrue: handleOpenModal,
    setFalse: handleCloseModal,
  } = useBoolean(false);

  const period: TimeParams | undefined = useMemo(() => {
    if (!data) return;
    const params = data[0]?.vmui?.params;

    const result = {
      start: +(params?.start || 0),
      end: +(params?.end || 0),
      step: params?.step,
      date: ""
    };

    if (!params) {
      const dataResult = data.filter(d => d.data.resultType === "matrix").map(d => d.data.result).flat();
      const times = dataResult.map(r => r.values ? r.values?.map(v => v[0]) : [0]).flat();
      const uniqTimes = Array.from(new Set(times.filter(Boolean))).sort((a, b) => a - b);
      result.start = uniqTimes[0];
      result.end = uniqTimes[uniqTimes.length - 1];
      result.step = humanizeSeconds(findMostCommonStep(uniqTimes));
    }

    result.date = formatDateToUTC(dateFromSeconds(result.end));
    return result;
  }, [data]);

  const isValidResponse = (response: unknown[]): boolean => {
    return response.every(element => {
      if (typeof element === "object" && element !== null) {
        const data = (element as { data?: unknown }).data;
        if (typeof data === "object" && data !== null) {
          const result = (data as { result?: unknown }).result;
          const resultType = (data as { resultType?: unknown }).resultType;
          return Array.isArray(result) && typeof resultType === "string";
        }
      }
      return false;
    });
  };

  const handleOnload = (result: string) => {
    try {
      const obj = JSON.parse(result);
      const response = Array.isArray(obj) ? obj : [obj];
      if (isValidResponse(response)) {
        setData(response);
      } else {
        setError("Invalid structure - JSON does not match the expected format");
      }
    } catch (e) {
      if (e instanceof Error) {
        setError(`${e.name}: ${e.message}`);
      }
    }
  };

  const handleReadFiles = (files: File[]) => {
    files.map(f => {
      const reader = new FileReader();
      reader.onload = (e) => {
        const result = String(e.target?.result);
        handleOnload(result);
      };
      reader.readAsText(f);
    });
  };

  const handleChange = (e: ChangeEvent<HTMLInputElement>) => {
    setError("");
    const files = Array.from(e.target.files || []);
    handleReadFiles(files);
    e.target.value = "";
  };

  const handleCloseError = () => {
    setError("");
  };

  const { files, dragging } = useDropzone();

  useEffect(() => {
    handleReadFiles(files);
  }, [files]);

  return (
    <div className="vm-trace-page">
      {hasData && (
        <div className="vm-trace-page-header">
          <div className="vm-trace-page-header-errors">
            <QueryAnalyzerInfo
              data={data}
              period={period}
            />
          </div>
          <div>
            <UploadJsonButtons
              onOpenModal={handleOpenModal}
              onChange={handleChange}
            />
          </div>
        </div>
      )}

      {error && (
        <div className="vm-trace-page-header-errors-item vm-trace-page-header-errors-item_margin-bottom">
          <Alert variant="error">{error}</Alert>
          <Button
            className="vm-trace-page-header-errors-item__close"
            startIcon={<CloseIcon/>}
            variant="text"
            color="error"
            onClick={handleCloseError}
          />
        </div>
      )}

      {hasData && (
        <QueryAnalyzerView
          data={data}
          period={period}
        />
      )}

      {!hasData && (
        <div className="vm-trace-page-preview">
          <p className="vm-trace-page-preview__text">
            Please, upload file with JSON response content.
            {"\n"}
            The file must contain query information in JSON format.
            {"\n"}
            Graph will be displayed after file upload.
            {"\n"}
            Attach files by dragging & dropping, selecting or pasting them.
          </p>
          <UploadJsonButtons
            onOpenModal={handleOpenModal}
            onChange={handleChange}
          />
        </div>
      )}

      {openModal && (
        <Modal
          title="Paste JSON"
          onClose={handleCloseModal}
        >
          <JsonForm
            onClose={handleCloseModal}
            onUpload={handleOnload}
          />
        </Modal>
      )}

      {dragging && <div className="vm-trace-page__dropzone"/>}
    </div>
  );
};

export default QueryAnalyzer;
