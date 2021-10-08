// eslint-disable-next-line @typescript-eslint/no-var-requires,no-undef
const {override, addExternalBabelPlugin} = require("customize-cra");

// eslint-disable-next-line no-undef
module.exports = override(
  addExternalBabelPlugin("@babel/plugin-proposal-nullish-coalescing-operator")
);