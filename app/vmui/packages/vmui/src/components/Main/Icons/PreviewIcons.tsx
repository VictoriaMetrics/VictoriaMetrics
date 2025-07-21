import { FC } from "preact/compat";
import * as icons from "./index";
import useCopyToClipboard from "../../../hooks/useCopyToClipboard";
import "./style.scss";

const PreviewIcons: FC = () => {
  const copyToClipboard = useCopyToClipboard();

  const createHandlerClickIcon = (key: string) => async () => {
    await copyToClipboard(`<${key}/>`, `<${key}/> has been copied`);
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
