import React, { FC, useEffect, useMemo, useState } from "preact/compat";
import { ChangeEvent } from "react";
import Trace from "../../components/TraceQuery/Trace";
import TracingsView from "../../components/TraceQuery/TracingsView";
import Tooltip from "../../components/Main/Tooltip/Tooltip";
import Button from "../../components/Main/Button/Button";
import Alert from "../../components/Main/Alert/Alert";
import "./style.scss";
import { CloseIcon } from "../../components/Main/Icons";
import Modal from "../../components/Main/Modal/Modal";
import JsonForm from "./JsonForm/JsonForm";
import { ErrorTypes } from "../../types";
import { setQueryStringWithoutPageReload } from "../../utils/query-string";

const TracePage: FC = () => {
  const [openModal, setOpenModal] = useState(false);
  const [tracesState, setTracesState] = useState<Trace[]>([]);
  const [errors, setErrors] = useState<{filename: string, text: string}[]>([]);
  const hasTraces = useMemo(() => !!tracesState.length, [tracesState]);

  const handleOpenModal = () => {
    setOpenModal(true);
  };

  const handleCloseModal = () => {
    setOpenModal(false);
  };

  const handleError = (e: Error, filename = "") => {
    setErrors(prev => [{ filename, text: `: ${e.message}` }, ...prev]);
  };

  const handleOnload = (result: string, filename: string) => {
    try {
      const resp = JSON.parse(result);
      const traceData = resp.trace || resp;
      if (!traceData.duration_msec) {
        handleError(new Error(ErrorTypes.traceNotFound), filename);
        return;
      }
      const trace = new Trace(traceData, filename);
      setTracesState(prev => [trace, ...prev]);
    } catch (e) {
      if (e instanceof Error) handleError(e, filename);
    }
  };

  const handleChange = (e: ChangeEvent<HTMLInputElement>) => {
    setErrors([]);
    const files = Array.from(e.target.files || []);
    files.map(f => {
      const reader = new FileReader();
      const filename = f?.name || "";
      reader.onload = (e) => {
        const result = String(e.target?.result);
        handleOnload(result, filename);
      };
      reader.readAsText(f);
    });
    e.target.value = "";
  };

  const handleTraceDelete = (trace: Trace) => {
    const updatedTraces = tracesState.filter((data) => data.idValue !== trace.idValue);
    setTracesState([...updatedTraces]);
  };

  const handleCloseError = (index: number) => {
    setErrors(prev => prev.filter((e,i) => i !== index));
  };

  const createHandlerCloseError = (index: number) => () => {
    handleCloseError(index);
  };

  useEffect(() => {
    setQueryStringWithoutPageReload({});
  }, []);

  const UploadButtons = () => (
    <div className="vm-trace-page-controls">
      <Button
        variant="outlined"
        onClick={handleOpenModal}
      >
        Paste JSON
      </Button>
      <Tooltip title="The file must contain tracing information in JSON format">
        <Button>
          Upload Files
          <input
            id="json"
            type="file"
            accept="application/json"
            multiple
            title=" "
            onChange={handleChange}
          />
        </Button>
      </Tooltip>
    </div>
  );

  return (
    <div className="vm-trace-page">
      <div className="vm-trace-page-header">
        <div className="vm-trace-page-header-errors">
          {errors.map((error, i) => (
            <div
              className="vm-trace-page-header-errors-item"
              key={`${error}_${i}`}
            >
              <Alert variant="error">
                <b className="vm-trace-page-header-errors-item__filename">{error.filename}</b>
                <span>{error.text}</span>
              </Alert>
              <Button
                className="vm-trace-page-header-errors-item__close"
                startIcon={<CloseIcon/>}
                variant="text"
                color="error"
                onClick={createHandlerCloseError(i)}
              />
            </div>
          ))}
        </div>
        <div>
          {hasTraces && <UploadButtons/>}
        </div>
      </div>

      {hasTraces && (
        <div>
          <TracingsView
            jsonEditor={true}
            traces={tracesState}
            onDeleteClick={handleTraceDelete}
          />
        </div>
      )}

      {!hasTraces && (
        <div className="vm-trace-page-preview">
          <p className="vm-trace-page-preview__text">
            Please, upload file with JSON response content.
            {"\n"}
            The file must contain tracing information in JSON format.
            {"\n"}
            In order to use tracing please refer to the doc:&nbsp;
            <a
              className="vm__link vm__link_colored"
              href="https://docs.victoriametrics.com/#query-tracing"
              target="_blank"
              rel="noreferrer"
            >
              https://docs.victoriametrics.com/#query-tracing
            </a>
            {"\n"}
            Tracing graph will be displayed after file upload.
          </p>
          <UploadButtons/>
        </div>
      )}

      {openModal && (
        <Modal
          title="Paste JSON"
          onClose={handleCloseModal}
        >
          <JsonForm
            editable
            displayTitle
            defaultTile={`JSON ${tracesState.length + 1}`}
            onClose={handleCloseModal}
            onUpload={handleOnload}
          />
        </Modal>
      )}
    </div>
  );
};

export default TracePage;
