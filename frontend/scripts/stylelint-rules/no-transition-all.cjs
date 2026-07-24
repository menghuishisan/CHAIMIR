/**
 * Stylelint rule that rejects broad, layout-triggering, and retired motion declarations.
 */

module.exports = function noTransitionAll({ root, result, ruleName, stylelint }) {
  const layoutProperties = /^(?:width|height|inline-size|block-size|inset|inset-inline|inset-block|left|right|top|bottom|margin(?:-.+)?|padding(?:-.+)?)\b/i

  root.walkDecls(/^transition(?:-property)?$/i, (decl) => {
    if (/\ball\b/i.test(decl.value)) {
      stylelint.utils.report({
        message: 'transition must name its animated properties; transition: all is forbidden.',
        node: decl,
        result,
        ruleName,
      })
    }

    const transitions = decl.value.split(',').map((value) => value.trim())
    if (transitions.some((value) => layoutProperties.test(value))) {
      stylelint.utils.report({
        message: 'Layout properties must not be transitioned; use transform or opacity for movement.',
        node: decl,
        result,
        ruleName,
      })
    }
  })

  root.walkDecls((decl) => {
    if (decl.prop === '--ease-in' || /var\(--ease-in\)/i.test(decl.value) || /--ease-spring(?:-ios|-bouncy)?\b/i.test(decl.value)) {
      stylelint.utils.report({
        message: 'Retired easing tokens are forbidden; use the shared motion tokens.',
        node: decl,
        result,
        ruleName,
      })
    }
  })
}
