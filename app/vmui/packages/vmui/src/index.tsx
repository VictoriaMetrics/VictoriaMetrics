import React, { render } from "preact/compat";
import "./constants/dayjsPlugins";
import App from "./App";
import reportWebVitals from "./reportWebVitals";
import "./styles/style.scss";

const root = document.getElementById("root");
if (root) render(<App />, root);


// If you want to start measuring performance in your app, pass a function
// to log results (for example: reportWebVitals(console.log))
// or send to an analytics endpoint. Learn more: https://bit.ly/CRA-vitals
reportWebVitals();
