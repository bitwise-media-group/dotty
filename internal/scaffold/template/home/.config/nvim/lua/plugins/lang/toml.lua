-- Copyright 2026 BitWise Media Group Ltd
-- SPDX-License-Identifier: MIT

return {
  recommended = function()
    return LazyVim.extras.wants({
      ft = "toml",
      root = "*.toml",
    })
  end,
  "neovim/nvim-lspconfig",
  opts = {
    servers = {
      tombi = {},
    },
  },
}
