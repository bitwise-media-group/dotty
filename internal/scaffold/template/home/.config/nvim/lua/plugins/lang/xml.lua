-- Copyright 2026 BitWise Media Group Ltd
-- SPDX-License-Identifier: MIT

return {
  {
    "nvim-treesitter/nvim-treesitter",
    opts = { ensure_installed = { "xml" } },
  },
  {
    "neovim/nvim-lspconfig",
    opts = {
      servers = {
        lemminx = {},
      },
    },
  },
}
