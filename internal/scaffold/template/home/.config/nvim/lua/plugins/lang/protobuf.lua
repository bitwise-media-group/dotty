-- Copyright 2026 BitWise Media Group Ltd
-- SPDX-License-Identifier: MIT

return {
  {
    "mason-org/mason.nvim",
    opts = { ensure_installed = { "buf" } },
  },
  {
    "neovim/nvim-lspconfig",
    opts = {
      servers = {
        buf_ls = {},
      },
    },
  },
  {
    "mfussenegger/nvim-lint",
    optional = true,
    opts = {
      linters_by_ft = {
        ["proto"] = { "buf_lint" },
      },
    },
  },
  {
    "stevearc/conform.nvim",
    optional = true,
    opts = {
      formatters_by_ft = {
        ["proto"] = { "buf" },
      },
    },
  },
}
