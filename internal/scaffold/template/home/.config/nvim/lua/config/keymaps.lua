-- Copyright 2026 BitWise Media Group Ltd
-- SPDX-License-Identifier: MIT

-- stylua: ignore
-- event: VeryLazy
-- defaults: https://github.com/LazyVim/LazyVim/blob/main/lua/lazyvim/config/keymaps.lua

local util = require("lazyvim.util")

local function map(mode, lhs, rhs, opts)
  local keys = require("lazy.core.handler").handlers.keys
  ---@cast keys LazyKeysHandler
  -- do not create the keymap if a lazy keys handler exists
  if not keys.active[keys.parse({ lhs, mode = mode }).id] then
    opts = opts or {}
    opts.silent = opts.silent ~= false
    vim.keymap.set(mode, lhs, rhs, opts)
  end
end

-- use tabs to switch between buffers
if util.has("bufferline.nvim") then
  map("n", "<S-tab>", "<cmd>BufferLineCyclePrev<cr>", { desc = "Prev buffer" })
  map("n", "<tab>", "<cmd>BufferLineCycleNext<cr>", { desc = "Next buffer" })
else
  map("n", "<S-tab>", "<cmd>bprevious<cr>", { desc = "Prev buffer" })
  map("n", "<tab>", "<cmd>bnext<cr>", { desc = "Next buffer" })
end

-- indentation in normal mode
map("n", "<", "<<", { desc = "Deindent" })
map("n", ">", ">>", { desc = "Indent" })

-- save without formatting
map("n", "<A-s>", "<cmd>noautocmd w<CR>", { desc = "Save Without Formatting" })

-- increment / decrement
map("n", "+", "<C-a>")
map("n", "-", "<C-x>")

-- redo the right way
map("n", "U", "<C-r>", { desc = "Redo" })

-- yank buffer
map("n", "<C-c>", ":%y+<CR>", { desc = "Yank Entire Buffer", silent = true })

-- select all
map("n", "<C-e>", "gg<S-V>G", { desc = "Select all Text", silent = true, noremap = true })

-- spelling dictionary
map("n", "<leader>!", "zg", { desc = "Add Word to Dictionary" })
map("n", "<leader>@", "zug", { desc = "Remove Word from Dictionary" })
