module.exports = {
  root: true,
  env: { browser: true, es2021: true, node: true },
  extends: [
    'eslint:recommended',
    'plugin:react/recommended',
    'plugin:react-hooks/recommended',
    'plugin:jsx-a11y/recommended',
    'prettier', // va último: desactiva reglas de estilo que ya resuelve Prettier
  ],
  parserOptions: {
    ecmaVersion: 'latest',
    sourceType: 'module',
    ecmaFeatures: { jsx: true },
  },
  settings: { react: { version: 'detect' } },
  ignorePatterns: ['dist', 'node_modules', 'coverage'],
  rules: {
    // React 17+/Vite con el runtime automático de JSX no necesita `import React`.
    'react/react-in-jsx-scope': 'off',
    'react/prop-types': 'off',
    // Deja avisar de variables sin usar, pero no bloquea el build por argumentos ignorados a propósito.
    'no-unused-vars': ['warn', { argsIgnorePattern: '^_' }],
  },
}
