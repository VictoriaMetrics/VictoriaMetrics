import Header from "../Header/Header";
import React, { FC } from "preact/compat";
import Box from "@mui/material/Box";
import { Outlet } from "react-router-dom";

const HomeLayout: FC = () => {
  return <Box>
    <Header/>
    <Outlet/>
  </Box>;
};

export default HomeLayout;
