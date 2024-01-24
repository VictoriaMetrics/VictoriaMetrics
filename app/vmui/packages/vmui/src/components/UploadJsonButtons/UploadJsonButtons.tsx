import React, { FC } from "preact/compat";
import { ChangeEvent } from "react";
import Button from "../Main/Button/Button";
import "./style.scss";

interface Props {
  onOpenModal: () => void;
  onChange: (e: ChangeEvent<HTMLInputElement>) => void;
}

const UploadJsonButtons: FC<Props> = ({ onOpenModal, onChange }) => (
  <div className="vm-upload-json-buttons">
    <Button
      variant="outlined"
      onClick={onOpenModal}
    >
      Paste JSON
    </Button>
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
  </div>
);

export default UploadJsonButtons;
