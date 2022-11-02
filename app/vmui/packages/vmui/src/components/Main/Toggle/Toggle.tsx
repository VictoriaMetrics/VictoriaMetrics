import React, { FC } from "preact/compat";
import Box from "@mui/material/Box";
import FormControlLabel from "@mui/material/FormControlLabel";
import BasicSwitch from "../../../theme/switch";

interface ToggleProps {
  label: string;
  value: boolean;
  onChange: () => void;
}

const Toggle: FC<ToggleProps> = ({ label, value, onChange }) => {

  return (
    <Box>
      <FormControlLabel
        label={label}
        sx={{ m: 0 }}
        control={(
          <BasicSwitch
            checked={value}
            onChange={onChange}
          />
        )}
      />
    </Box>
  );
};

export default Toggle;
