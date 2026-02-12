module.exports = {
  root: true,
  env: {
    browser: true,
    es2022: true,
    node: true,
  },
  parser: "@typescript-eslint/parser",
  parserOptions: {
    ecmaVersion: "latest",
    sourceType: "module",
    ecmaFeatures: {
      jsx: true,
    },
  },
  plugins: ["@typescript-eslint", "react-hooks", "react-refresh"],
  extends: ["plugin:react-hooks/recommended"],
  rules: {
    "react-refresh/only-export-components": "warn",
  },
  overrides: [
    {
      files: [
        "src/components/editor/MonacoEditor.tsx",
        "src/components/ide/CodeComments.tsx",
        "src/components/mobile/MobileNavigation.tsx",
        "src/components/ui/*.tsx",
        "src/components/ui/**/*.tsx",
      ],
      rules: {
        "react-refresh/only-export-components": "off",
      },
    },
  ],
  ignorePatterns: ["dist", "node_modules"],
};
