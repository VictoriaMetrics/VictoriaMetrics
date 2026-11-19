import { FC, useMemo } from "preact/compat";
import "./style.scss";
import { createFaviconUrl } from "../../../../utils/favicon";
import classNames from "classnames";
import { useBrowserTabSync } from "./hooks/useBrowserTabSync";
import { CloseIcon } from "../../../Main/Icons";
import { faviconColors } from "../../../../constants/faviconColors";

const BrowserTabController: FC = () => {
  const { faviconColor, changeFaviconColor } = useBrowserTabSync();

  const faviconUrl = useMemo(() => {
    return createFaviconUrl(faviconColor);
  }, [faviconColor]);

  const createHandlerClick = (color?: string) => () => {
    changeFaviconColor(color);
  };

  return (
    <div className="vm-browser-tab-controller">
      <p className="vm-server-configurator__title">Favicon color</p>

      <div className="vm-browser-tab-controller-palette">
        <div className="vm-browser-tab-controller-palette-list">
          <button
            className="vm-browser-tab-controller-palette-list__item vm-browser-tab-controller-palette-list__item_reset"
            type="button"
            onClick={createHandlerClick()}
            aria-label="Reset favicon color"
          >
            <CloseIcon />
          </button>

          {faviconColors.map(color => (
            <button
              className={classNames({
                "vm-browser-tab-controller-palette-list__item": true,
                "vm-browser-tab-controller-palette-list__item_selected": faviconColor === color
              })}
              key={color}
              type="button"
              style={{ color }}
              onClick={createHandlerClick(color)}
              aria-label={`Set favicon color to ${color}`}
              aria-pressed={faviconColor === color}
            />
          ))}
        </div>

        <img
          className="vm-browser-tab-controller-palette__preview"
          src={faviconUrl}
          alt="Favicon preview"
        />
      </div>
    </div>
  );
};

export default BrowserTabController;
