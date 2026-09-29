import { FC } from "preact/compat";
import useBoolean from "../../../hooks/useBoolean";
import classNames from "classnames";
import TextField from "../TextField/TextField";
import "./style.scss";
import SafeHtml from "../SafeHtml/SafeHtml";

interface Props {
  value: string;
  onChange: (value: string) => void;
}

const tabs = [
  { title: "Write", value: false },
  { title: "Preview", value: true },
];

const MarkdownEditor: FC<Props> = ({ value, onChange }) => {
  const {
    value: markdownPreview,
    setTrue: setMarkdownPreviewTrue,
    setFalse: setMarkdownPreviewFalse,
  } = useBoolean(false);

  return (
    <div className="vm-markdown-editor">
      <div className="vm-markdown-editor-header">
        <div className="vm-markdown-editor-header-tabs">
          {tabs.map(({ title, value }) => (
            <div
              key={title}
              className={classNames({
                "vm-markdown-editor-header-tabs__tab": true,
                "vm-markdown-editor-header-tabs__tab_active": markdownPreview === value,
              })}
              onClick={value ? setMarkdownPreviewTrue : setMarkdownPreviewFalse}
            >
              {title}
            </div>
          ))}
        </div>
        <span className="vm-markdown-editor-header__info">
          Markdown is supported
        </span>
      </div>
      {markdownPreview ? (
        <SafeHtml
          tagName="div"
          className="vm-markdown-editor-preview vm-markdown"
          value={value}
          format="markdown"
        />
      ) : (
        <TextField
          type="textarea"
          value={value}
          onChange={onChange}
        />
      )}
    </div>
  );
};

export default MarkdownEditor;
