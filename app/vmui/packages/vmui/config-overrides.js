/* eslint-disable */
const {override, addExternalBabelPlugin, addWebpackAlias, addWebpackPlugin} = require("customize-cra");
const webpack = require("webpack");
const fs = require('fs');
const path = require('path');

// This will replace the default check
if (process.env.REACT_APP_TYPE === "logs") {
  const fileContent = fs.readFileSync(path.resolve(__dirname, 'public/vl_index.html'), 'utf8');
  fs.writeFileSync(path.resolve(__dirname, 'public/index.html'), fileContent);
} else {
  const fileContent = fs.readFileSync(path.resolve(__dirname, 'public/vm_index.html'), 'utf8');
  fs.writeFileSync(path.resolve(__dirname, 'public/index.html'), fileContent);
}

module.exports = override(
  addExternalBabelPlugin("@babel/plugin-proposal-nullish-coalescing-operator"),
  addWebpackAlias({
    "react": "preact/compat",
    "react-dom/test-utils": "preact/test-utils",
    "react-dom": "preact/compat", // Must be below test-utils
    "react/jsx-runtime": "preact/jsx-runtime"
  }),
  addWebpackPlugin(
    new webpack.NormalModuleReplacementPlugin(
      /\.\/App/,
      function (resource) {
        if (process.env.REACT_APP_TYPE === "logs") {
          resource.request = "./AppLogs";
        }
        if (process.env.REACT_APP_TYPE === "anomaly") {
          resource.request = "./AppAnomaly";
        }
      }
    )
  )
);
