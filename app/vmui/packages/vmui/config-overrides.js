// eslint-disable-next-line @typescript-eslint/no-var-requires,no-undef
const {override, addExternalBabelPlugin, addWebpackAlias} = require("customize-cra");

// eslint-disable-next-line no-undef
module.exports = override(
  addExternalBabelPlugin("@babel/plugin-proposal-nullish-coalescing-operator"),
  addWebpackAlias({
    "react": "preact/compat",
    "react-dom/test-utils": "preact/test-utils",
    "react-dom": "preact/compat", // Must be below test-utils
    "react/jsx-runtime": "preact/jsx-runtime"
  })
);