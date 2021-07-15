import React, {FC} from "react";
import {Box, IconButton, Tooltip} from "@material-ui/core";
import FileCopyIcon from "@material-ui/icons/FileCopy";
import {useSnack} from "../../contexts/Snackbar";

interface UrlCopyProps {
  url?: string
}

export const UrlCopy: FC<UrlCopyProps> = ({url}) => {

  const {showInfoMessage} = useSnack();

  return <Box pl={2} py={1} flexShrink={0} display="flex">
    <Tooltip title="Copy Query URL">
      <IconButton size="small" onClick={(e) => {
        if (url) {
          navigator.clipboard.writeText(url);
          showInfoMessage("Value has been copied");
          e.preventDefault(); // needed to avoid snackbar immediate disappearing
        }
      }}>
        <FileCopyIcon style={{color: "white"}}/>
      </IconButton>
    </Tooltip>
  </Box>;
};