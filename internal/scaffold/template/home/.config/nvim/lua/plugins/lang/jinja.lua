-- Copyright 2026 BitWise Media Group Ltd
-- SPDX-License-Identifier: MIT

return {
  {
    "nvim-treesitter/nvim-treesitter",
    opts = { ensure_installed = { "jinja" } },
  },
  {
    "mason-org/mason.nvim",
    opts = { ensure_installed = { "djlint" } },
  },
  {
    "mfussenegger/nvim-lint",
    optional = true,
    opts = {
      linters_by_ft = {
        ["htmldjango"] = { "djlint" },
      },
    },
  },
  {
    "stevearc/conform.nvim",
    optional = true,
    opts = {
      formatters_by_ft = {
        ["htmldjango"] = { "djlint", "prettier" },
      },
    },
  },
}
