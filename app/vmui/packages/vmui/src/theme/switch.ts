import {styled} from "@mui/material/styles";
import Switch from "@mui/material/Switch";

const BasicSwitch = styled(Switch)(() => ({
  padding: 10,
  "& .MuiSwitch-track": {
    borderRadius: 14,
    "&:before, &:after": {
      content: "\"\"",
      position: "absolute",
      top: "50%",
      transform: "translateY(-50%)",
      width: 14,
      height: 14,
    },
  },
  "& .MuiSwitch-thumb": {
    boxShadow: "none",
    width: 12,
    height: 12,
    margin: 4,
  },
}));

export default BasicSwitch;