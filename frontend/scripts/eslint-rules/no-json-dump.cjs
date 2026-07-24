/**
 * ESLint rule that rejects raw JSON dumps in user-facing JSX.
 */

module.exports = {
  meta: { type: 'problem', docs: { description: 'Disallow JSON.stringify dumps in pre elements.' }, schema: [] },
  create(context) {
    function isJsonStringify(node) {
      return node?.type === 'CallExpression'
        && node.callee?.type === 'MemberExpression'
        && node.callee.object?.type === 'Identifier'
        && node.callee.object.name === 'JSON'
        && node.callee.property?.type === 'Identifier'
        && node.callee.property.name === 'stringify'
    }

    return {
      JSXElement(node) {
        if (node.openingElement.name?.type !== 'JSXIdentifier' || node.openingElement.name.name !== 'pre') return
        const hasDump = node.children.some((child) => child.type === 'JSXExpressionContainer' && isJsonStringify(child.expression))
        if (hasDump) context.report({ node, message: 'Render structured data with a semantic UI primitive instead of a raw JSON dump.' })
      },
    }
  },
}
