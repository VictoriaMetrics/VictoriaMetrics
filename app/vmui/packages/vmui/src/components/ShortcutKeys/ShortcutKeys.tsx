import React, {FC, useState} from "preact/compat";
import Tooltip from "@mui/material/Tooltip";
import Button from "@mui/material/Button";
import Modal from "@mui/material/Modal";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import IconButton from "@mui/material/IconButton";
import KeyboardIcon from "@mui/icons-material/Keyboard";
import CloseIcon from "@mui/icons-material/Close";
import {isMacOs} from "../../utils/detect-os";

const modalStyle = {
  position: "absolute" as const,
  top: "50%",
  left: "50%",
  transform: "translate(-50%, -50%)",
  bgcolor: "background.paper",
  p: 3,
  borderRadius: "4px",
  minWidth: "300px",
  maxWidth: "800px"
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
        keys: [ctrlMeta, "Arrow Up"],
        description: "Previous command from the Query history"
      },
      {
        keys: [ctrlMeta, "Arrow Down"],
        description: " Next command from the Query history"
      }
    ]
  },
  {
    title: "Graph",
    list: [
      {
        keys: [ctrlMeta, "Arrow Down"],
        description: "Run"
      }
    ]
  },
  {
    title: "Legend",
    list: [
      {
        keys: [ctrlMeta, "Arrow Down"],
        description: "Run"
      }
    ]
  }
];

const ShortcutKeys: FC = () => {
  const [openList, setOpenList] = useState(false);

  return <>
    <Tooltip title={"Shortcut keys"}>
      <Button variant="contained" color="primary"
        sx={{
          color: "white",
          border: "1px solid rgba(0, 0, 0, 0.2)",
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
            <Box key={section.title} mb={2}>
              <Typography variant="body2" component="h3" fontWeight="bold" mb={1}>
                {section.title}
              </Typography>
              <Box>
                {section.list.map(l => (
                  <Box key={l.keys.join("+")} display="grid" gridTemplateColumns="150px 1fr" alignItems="center" mb={1}>
                    <Box display="flex" alignItems="center" fontSize="10px" gap={"4px"}>
                      {l.keys.map((k, i) => <>
                        <code key={k} className="shortcut-key">{k}</code> {i !== l.keys.length - 1 ? "+" : ""}
                      </>)}
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
