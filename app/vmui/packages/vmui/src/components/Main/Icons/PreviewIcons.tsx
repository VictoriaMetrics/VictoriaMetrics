import React, { FC } from "preact/compat";
import * as icons from "./index";
import { useSnack } from "../../../contexts/Snackbar";
import "./style.scss";

const PreviewIcons: FC = () => {
  const { showInfoMessage } = useSnack();

  const handleClickIcon = (copyValue: string) => {
    navigator.clipboard.writeText(`<${copyValue}/>`);
    showInfoMessage({ text: `<${copyValue}/> has been copied`, type: "success" });
  };

  const createHandlerClickIcon = (key: string) => () => {
    handleClickIcon(key);
  };

  return (
    <div className="vm-preview-icons">
      {Object.entries(icons).map(([iconKey, icon]) => (
        <div
          className="vm-preview-icons-item"
          onClick={createHandlerClickIcon(iconKey)}
          key={iconKey}
        >
          <div className="vm-preview-icons-item__svg">
            {icon()}
          </div>
          <div className="vm-preview-icons-item__name">
            {`<${iconKey}/>`}
          </div>
        </div>
      ))}
    </div>
  );
};

export default PreviewIcons;
