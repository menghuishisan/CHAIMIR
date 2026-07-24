/**
 * Stylelint rule that prevents removing focus indication without a replacement.
 */

function hasFocusVisibleRule(root, selector) {
  if (selector.includes(':focus:not(:focus-visible)')) return true
  const classes = selector.match(/\.[A-Za-z_][\w-]*/g) ?? []
  let found = false
  root.walkRules((rule) => {
    if (found || !rule.selector.includes(':focus-visible')) return
    found = classes.length === 0
      ? true
      : classes.some((className) => rule.selector.includes(`${className}:focus-visible`))
  })
  return found
}

module.exports = function noOutlineNoneWithoutFocusVisible({ root, result, ruleName, stylelint }) {
  root.walkDecls(/^outline$/i, (decl) => {
    if (!/^\s*(?:none|0)\s*$/i.test(decl.value)) return
    const selector = decl.parent.type === 'rule' ? decl.parent.selector : ''
    if (hasFocusVisibleRule(root, selector)) return

    stylelint.utils.report({
      message: 'outline: none requires a matching :focus-visible focus treatment.',
      node: decl,
      result,
      ruleName,
    })
  })
}
