/**
 * Chaimir's local ESLint plugin rules.
 */

module.exports = {
  rules: {
    'no-emoji': require('./no-emoji.cjs'),
    'no-json-dump': require('./no-json-dump.cjs'),
  },
}
