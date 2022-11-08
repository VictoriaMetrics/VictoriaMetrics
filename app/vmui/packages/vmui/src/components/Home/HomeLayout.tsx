import Header from "../Header/Header";
import React, { FC } from "preact/compat";
import { Outlet } from "react-router-dom";

const HomeLayout: FC = () => {
  return <section>
    <Header/>
    <Outlet/>
  </section>;
};

export default HomeLayout;
