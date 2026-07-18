-- Copyright 2026 BitWise Media Group Ltd
-- SPDX-License-Identifier: MIT

return {
  {
    "neovim/nvim-lspconfig",
    opts = {
      servers = {
        gopls = {
          init_options = {
            semanticTokens = true,
          },
          settings = {
            gopls = {
              directoryFilters = { "-.git", "-.vscode", "-.idea", "-.vscode-test", "-node_modules", "-**/.terraform" },
            },
          },
        },
      },
    },
  },
  {
    "stevearc/conform.nvim",
    optional = true,
    opts = {
      formatters_by_ft = {
        ["go"] = { "gofumpt" },
      },
    },
  },
}
