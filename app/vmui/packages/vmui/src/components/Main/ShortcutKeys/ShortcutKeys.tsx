import React, { FC, useState } from "preact/compat";
import { isMacOs } from "../../../utils/detect-os";
import { getAppModeEnable } from "../../../utils/app-mode";
import Button from "../Button/Button";
import { KeyboardIcon } from "../Icons";
import Modal from "../Modal/Modal";
import "./style.scss";
import Tooltip from "../Tooltip/Tooltip";

const ctrlMeta = isMacOs() ? "Cmd" : "Ctrl";

const keyList = [
  {
    title: "Query",
    list: [
      {
        keys: ["Enter"],
        description: "Run"
      },
      {
        keys: ["Shift", "Enter"],
        description: "Multi-line queries"
      },
      {
        keys: [ctrlMeta, "Arrow Up"],
        description: "Previous command from the Query history"
      },
      {
        keys: [ctrlMeta, "Arrow Down"],
        description: "Next command from the Query history"
      }
    ]
  },
  {
    title: "Graph",
    list: [
      {
        keys: [ctrlMeta, "Scroll Up"],
        description: "Zoom in"
      },
      {
        keys: [ctrlMeta, "Scroll Down"],
        description: "Zoom out"
      },
      {
        keys: [ctrlMeta, "Click and Drag"],
        description: "Move the graph left/right"
      },
    ]
  },
  {
    title: "Legend",
    list: [
      {
        keys: ["Mouse Click"],
        description: "Select series"
      },
      {
        keys: [ctrlMeta, "Mouse Click"],
        description: "Toggle multiple series"
      }
    ]
  }
];

const ShortcutKeys: FC = () => {
  const [openList, setOpenList] = useState(false);
  const appModeEnable = getAppModeEnable();

  const handleOpen = () => {
    setOpenList(true);
  };

  const handleClose = () => {
    setOpenList(false);
  };

  return <>
    <Tooltip
      title="Shortcut keys"
      placement="bottom-center"
    >
      <Button
        className={appModeEnable ? "" : "vm-header-button"}
        variant="contained"
        color="primary"
        startIcon={<KeyboardIcon/>}
        onClick={handleOpen}
      />
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
              <h3 className="vm-shortcuts-section__title">
                {section.title}
              </h3>
              <div className="vm-shortcuts-section-list">
                {section.list.map(l => (
                  <div
                    className="vm-shortcuts-section-list-item"
                    key={l.keys.join("+")}
                  >
                    <div className="vm-shortcuts-section-list-item__key">
                      {l.keys.map((k, i) => (
                        <>
                          <code key={k}>
                            {k}
                          </code>
                          {i !== l.keys.length - 1 ? "+" : ""}
                        </>
                      ))}
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
