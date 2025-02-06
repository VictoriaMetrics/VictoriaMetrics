import React, { render } from "preact/compat";
import "./constants/dayjsPlugins";
import App from "./App";
import reportWebVitals from "./reportWebVitals";
import "./styles/style.scss";
import { APP_TYPE, AppType } from "./constants/appType";
import AppLogs from "./AppLogs";
import AppAnomaly from "./AppAnomaly";

const getAppComponent = () => {
  switch (APP_TYPE) {
    case AppType.victorialogs:
      return <AppLogs/>;
    case AppType.vmanomaly:
      return <AppAnomaly/>;
    default:
      return <App/>;
  }
};

const root = document.getElementById("root");
if (root) render(getAppComponent(), root);


// If you want to start measuring performance in your app, pass a function
// to log results (for example: reportWebVitals(console.log))
// or send to an analytics endpoint. Learn more: https://bit.ly/CRA-vitals
reportWebVitals();
