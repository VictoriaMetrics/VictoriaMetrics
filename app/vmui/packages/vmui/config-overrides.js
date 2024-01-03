/* eslint-disable */
const { override, addExternalBabelPlugin, addWebpackAlias, addWebpackPlugin } = require("customize-cra");
const webpack = require("webpack");

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
