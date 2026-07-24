/**
 * Single PostCSS source of truth. Global token data is injected before custom
 * media expansion so CSS Modules and package CSS receive the same breakpoints.
 */

const path = require('node:path')

module.exports = {
  plugins: [
    require('@csstools/postcss-global-data')({
      files: [path.resolve(__dirname, '../packages/ui/src/tokens/breakpoints.css')],
    }),
    require('postcss-custom-media')({ preserve: false }),
  ],
}
