import React, {FC} from "react";
import {Box, Button, Grid, Typography} from "@material-ui/core";
import {useSnack} from "../../contexts/Snackbar";

interface UrlLineProps {
  url?: string
}

export const UrlLine: FC<UrlLineProps> = ({url}) => {

  const {showInfoMessage} = useSnack();

  return <Grid item style={{backgroundColor: "#eee", width: "100%"}}>
    <Box flexDirection="row" display="flex" justifyContent="space-between" alignItems="center">
      <Box pl={2} py={1} display="flex" style={{
        flex: 1,
        minWidth: 0
      }}>
        <Typography style={{
          whiteSpace: "nowrap",
          overflow: "hidden",
          textOverflow: "ellipsis",
          fontStyle: "italic",
          fontSize: "small",
          color: "#555"
        }}>
          Currently showing {url}
        </Typography>

      </Box>
      <Box px={2} py={1} flexShrink={0} display="flex">
        <Button size="small" onClick={(e) => {
          if (url) {
            navigator.clipboard.writeText(url);
            showInfoMessage("Value has been copied");
            e.preventDefault(); // needed to avoid snackbar immediate disappearing
          }
        }}>Copy Query Url</Button>
      </Box>
    </Box>
  </Grid>;
};