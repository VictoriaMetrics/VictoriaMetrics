import React, { FC, useState } from "preact/compat";
import { isMacOs } from "../../../utils/detect-os";
import { getAppModeEnable } from "../../../utils/app-mode";
import Button from "../Button/Button";
import { KeyboardIcon } from "../Icons";
import Modal from "../Modal/Modal";

const modalStyle = {
  position: "absolute" as const,
  top: "50%",
  left: "50%",
  p: 3,
  minWidth: "300px",
  maxWidth: "800px",
  borderRadius: "4px",
  bgcolor: "background.paper",
  transform: "translate(-50%, -50%)",
};

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

  // sx={{
  //   color: "white",
  //     border: appModeEnable ? "none" : "1px solid rgba(0, 0, 0, 0.2)",
  //     minWidth: "34px",
  //     padding: "6px 8px",
  //     boxShadow: "none",
  // }}

  return <>
    {/*<Tooltip title={"Shortcut keys"}>*/}
    <Button
      variant="contained"
      color="primary"
      onClick={() => setOpenList(prev => !prev)}
    >
      <KeyboardIcon/>
    </Button>
    {/*</Tooltip>*/}

    {openList && (
      <Modal
        title={"Shortcut keys"}
        onClose={() => setOpenList(false)}
      >
        <div>
          {keyList.map(section => (
            <div
              key={section.title}
            >
              <h3>
                {section.title}
              </h3>
              {/*<Divider sx={{ mb: 1 }}/>*/}
              <div>
                {section.list.map(l => (
                  <div
                    key={l.keys.join("+")}
                  >
                    <div>
                      {l.keys.map((k, i) => (
                        <>
                          <code
                            key={k}
                            className="shortcut-key"
                          >{k}</code> {i !== l.keys.length - 1 ? "+" : ""}
                        </>
                      ))}
                    </div>
                    <p>
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
