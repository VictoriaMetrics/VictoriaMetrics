import Header from "../Header/Header";
import React, { FC } from "preact/compat";
import { Outlet } from "react-router-dom";
import "./style.scss";

const HomeLayout: FC = () => {
  return <section className="vm-container">
    <Header/>
    <div className="vm-container-body">
      <Outlet/>
    </div>
  </section>;
};

export default HomeLayout;
