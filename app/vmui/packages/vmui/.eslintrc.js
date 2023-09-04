// eslint-disable-next-line no-undef
module.exports = {
  "env": {
    "browser": true,
    "es2021": true
  },
  "extends": [
    "eslint:recommended",
    "plugin:react/recommended",
    "plugin:@typescript-eslint/recommended"
  ],
  "parser": "@typescript-eslint/parser",
  "parserOptions": {
    "ecmaFeatures": { "jsx": true },
    "ecmaVersion": 12,
    "sourceType": "module"
  },
  "plugins": [
    "react",
    "@typescript-eslint"
  ],
  "rules": {
    "react/jsx-closing-bracket-location": [1, "line-aligned"],
    "react/jsx-max-props-per-line":[1, { "maximum": 1 }],
    "react/jsx-first-prop-new-line": [1, "multiline"],
    "object-curly-spacing": [2, "always"],
    "indent": ["error", 2, { "SwitchCase": 1 }],
    "linebreak-style": ["error", "unix"],
    "quotes": ["error", "double"],
    "semi": ["error", "always"],
    "react/prop-types": 0
  },
  "settings": {
    "react": {
      "pragma": "React",  // Pragma to use, default to "React"
      "version": "detect"
    },
    "linkComponents": [
      // Components used as alternatives to <a> for linking, eg. <Link to={ url } />
      "Hyperlink",
      {
        "name": "Link", "linkAttribute": "to"
      }
    ]
  }
};

