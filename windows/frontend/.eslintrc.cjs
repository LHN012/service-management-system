module.exports = {
  env: { browser: true, es2021: true },
  extends: ['eslint:recommended'],
  parserOptions: { ecmaVersion: 'latest', sourceType: 'module', ecmaFeatures: { jsx: true } },
  plugins: ['react', 'react-hooks'],
  rules: {
    ...require('eslint-plugin-react-hooks').configs.recommended.rules,
    'react/jsx-uses-vars': 'error',
    'no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
  },
}
