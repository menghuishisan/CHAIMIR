/**
 * Stylelint rule that keeps raw colors in the token source of truth only.
 */

module.exports = function noRawHex({ root, result, ruleName, stylelint }) {
  const file = root.source?.input?.file?.replaceAll('\\', '/') ?? ''
  if (file.includes('/tokens/')) return

  root.walkDecls((decl) => {
    if (/#(?:[\da-f]{3,4}|[\da-f]{6}|[\da-f]{8})\b|\b(?:rgb|rgba|hsl|hsla)\s*\(/i.test(decl.value)) {
      stylelint.utils.report({
        message: 'Raw hex colors are forbidden outside the token source.',
        node: decl,
        result,
        ruleName,
      })
    }
  })
}
