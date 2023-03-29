import React, { FC } from "preact/compat";
import Button from "../../../components/Main/Button/Button";
import Tooltip from "../../../components/Main/Tooltip/Tooltip";
import { ChangeEvent } from "react";

interface TraceUploadButtonsProps {
  onOpenModal: () => void;
  onChange: (e: ChangeEvent<HTMLInputElement>) => void;
}

const TraceUploadButtons: FC<TraceUploadButtonsProps> = ({ onOpenModal, onChange }) => (
  <div className="vm-trace-page-controls">
    <Button
      variant="outlined"
      onClick={onOpenModal}
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
          onChange={onChange}
        />
      </Button>
    </Tooltip>
  </div>
);

export default TraceUploadButtons;
