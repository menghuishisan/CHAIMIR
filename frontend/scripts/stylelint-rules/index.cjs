/**
 * Chaimir's shared Stylelint plugin. Individual checks stay in separate files
 * so each gate has one responsibility while all workspaces load one plugin.
 */

const stylelint = require('stylelint')
const checks = [
  require('./no-transition-all.cjs'),
  require('./require-hover-pointer-gate.cjs'),
  require('./no-raw-hex.cjs'),
  require('./allowed-media-queries.cjs'),
  require('./no-outline-none-without-focus-visible.cjs'),
  require('./no-raw-z-index.cjs'),
  require('./require-animation-budget-over-300ms.cjs'),
]

const ruleName = 'chaimir/gates'
const plugin = stylelint.createPlugin(ruleName, (primaryOption, secondaryOptions, context) => (root, result) => {
  for (const check of checks) check({ root, result, ruleName, stylelint, context })
})

plugin.messages = stylelint.utils.ruleMessages(ruleName, {
  violation: (message) => message,
})

module.exports = plugin
