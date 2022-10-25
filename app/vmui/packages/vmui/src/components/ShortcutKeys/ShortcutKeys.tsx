import React, {FC, useState} from "preact/compat";
import Tooltip from "@mui/material/Tooltip";
import Button from "@mui/material/Button";
import Modal from "@mui/material/Modal";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import IconButton from "@mui/material/IconButton";
import KeyboardIcon from "@mui/icons-material/Keyboard";
import CloseIcon from "@mui/icons-material/Close";
import Divider from "@mui/material/Divider";
import {isMacOs} from "../../utils/detect-os";
import {getAppModeEnable} from "../../utils/app-mode";

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

  return <>
    <Tooltip title={"Shortcut keys"}>
      <Button variant="contained" color="primary"
        sx={{
          color: "white",
          border: appModeEnable ? "none" : "1px solid rgba(0, 0, 0, 0.2)",
          minWidth: "34px",
          padding: "6px 8px",
          boxShadow: "none",
        }}
        startIcon={<KeyboardIcon style={{marginRight: "-8px", marginLeft: "4px"}}/>}
        onClick={() => setOpenList(prev => !prev)}>
      </Button>
    </Tooltip>
    <Modal open={openList} onClose={() => setOpenList(false)}>
      <Box sx={modalStyle}>
        <Box display="grid" gridTemplateColumns="1fr auto" alignItems="center" mb={2}>
          <Typography id="modal-modal-title" variant="h6" component="h2">
            Shortcut keys
          </Typography>
          <IconButton size="small" onClick={() => setOpenList(false)}>
            <CloseIcon/>
          </IconButton>
        </Box>
        <Box>
          {keyList.map(section => (
            <Box key={section.title} mb={3}>
              <Typography variant="body1" component="h3" fontWeight="bold" mb={0.5}>
                {section.title}
              </Typography>
              <Divider sx={{mb: 1}}/>
              <Box>
                {section.list.map(l => (
                  <Box
                    key={l.keys.join("+")}
                    display="grid"
                    gridTemplateColumns="160px 1fr"
                    alignItems="center"
                    mb={1}
                  >
                    <Box display="flex" alignItems="center" fontSize="10px" gap={"4px"}>
                      {l.keys.map((k, i) => (
                        <>
                          <code key={k} className="shortcut-key">{k}</code> {i !== l.keys.length - 1 ? "+" : ""}
                        </>
                      ))}
                    </Box>
                    <Typography variant="body2" component="p">
                      {l.description}
                    </Typography>
                  </Box>
                ))}
              </Box>
            </Box>
          ))}
        </Box>
      </Box>
    </Modal>
  </>;
};

export default ShortcutKeys;
