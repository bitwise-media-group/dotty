-- Copyright 2026 BitWise Media Group Ltd
-- SPDX-License-Identifier: MIT

return {
  "MagicDuck/grug-far.nvim",
  keys = {
    {
      "<leader>sf",
      function()
        local grug = require("grug-far")
        local buffer = vim.fn.expand("%")
        grug.open({
          transient = true,
          prefills = {
            paths = buffer,
          },
        })
      end,
      mode = { "n", "v" },
      desc = "Search and Replace in Buffer",
    },
  },
}
