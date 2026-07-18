-- Copyright 2026 BitWise Media Group Ltd
-- SPDX-License-Identifier: MIT

return {
  "Pocco81/auto-save.nvim",
  lazy = false,
  opts = {
    trigger_events = { "InsertLeave" },
    debounce_delay = 500,
    execution_message = {
      message = function()
        return ""
      end,
    },
  },
  keys = {
    { "<leader>uv", "<cmd>ASToggle<CR>", desc = "Toggle Auto-save" },
  },
}
