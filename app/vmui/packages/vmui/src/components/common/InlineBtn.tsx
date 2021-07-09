import {makeStyles} from "@material-ui/core/styles";
import React from "react";
import {Link} from "@material-ui/core";

const useStyles = makeStyles({
  inlineBtn: {
    "&:hover": {
      cursor: "pointer"
    },
  }
});

export const InlineBtn: React.FC<{handler: () => void; text: string}> = ({handler, text}) => {
  const classes = useStyles();
  return <Link component="span" className={classes.inlineBtn}
    onClick={handler}>
    {text}
  </Link>;
};