import { FC, useCallback } from "preact/compat";
import { getAppModeEnable } from "../../../utils/app-mode";
import Button from "../Button/Button";
import { KeyboardIcon } from "../Icons";
import Modal from "../Modal/Modal";
import "./style.scss";
import Tooltip from "../Tooltip/Tooltip";
import keyList from "./constants/keyList";
import { isMacOs } from "../../../utils/detect-device";
import useBoolean from "../../../hooks/useBoolean";
import useEventListener from "../../../hooks/useEventListener";

const title = "Shortcut keys";
const isMac = isMacOs();
const keyOpenHelp = isMac ? "Cmd + /" : "F1";

const ShortcutKeys: FC<{ showTitle?: boolean }> = ({ showTitle }) => {
  const appModeEnable = getAppModeEnable();

  const {
    value: openList,
    setTrue: handleOpen,
    setFalse: handleClose,
  } = useBoolean(false);

  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    const openOnMac = isMac && e.key === "/" && e.metaKey;
    const openOnOther = !isMac && e.key === "F1" && !e.metaKey;
    if (openOnMac || openOnOther) {
      handleOpen();
    }
  }, [handleOpen]);

  useEventListener("keydown", handleKeyDown);

  return <>
    <Tooltip
      open={showTitle === true ? false : undefined}
      title={`${title} (${keyOpenHelp})`}
      placement="bottom-center"
    >
      <Button
        className={appModeEnable ? "" : "vm-header-button"}
        variant="contained"
        color="primary"
        startIcon={<KeyboardIcon/>}
        onClick={handleOpen}
        ariaLabel={title}
      >
        {showTitle && title}
      </Button>
    </Tooltip>

    {openList && (
      <Modal
        title={"Shortcut keys"}
        onClose={handleClose}
      >
        <div className="vm-shortcuts">
          {keyList.map(section => (
            <div
              className="vm-shortcuts-section"
              key={section.title}
            >
              {section.readMore && (
                <div className="vm-shortcuts-section__read-more">{section.readMore}</div>
              )}
              <h3 className="vm-shortcuts-section__title">
                {section.title}
              </h3>
              <div className="vm-shortcuts-section-list">
                {section.list.map((l, i) => (
                  <div
                    className="vm-shortcuts-section-list-item"
                    key={`${section.title}_${i}`}
                  >
                    <div className="vm-shortcuts-section-list-item__key">
                      {l.keys}
                    </div>
                    <p className="vm-shortcuts-section-list-item__description">
                      {l.description}
                    </p>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      </Modal>
    )}
  </>;
};

export default ShortcutKeys;
