-- Copyright 2026 BitWise Media Group Ltd
-- SPDX-License-Identifier: MIT

-- event: BeforeLazy
-- defaults: https://github.com/LazyVim/LazyVim/blob/main/lua/lazyvim/config/options.lua

local g = vim.g
local o = vim.opt

-- optimise startup
vim.loader.enable()
g.loaded_python3_provider = 0
g.loaded_perl_provider = 0
g.loaded_ruby_provider = 0
g.loaded_node_provider = 0

-- defaults
g.lualine_info_extras = false
g.lazyvim_cmp = "auto"
g.lazyvim_picker = "snacks"

-- formatting
g.autoformat = true

-- editor config
g.editorconfig = true

-- clipboard: yank to the *local* machine over SSH via OSC 52.
--
-- LazyVim blanks 'clipboard' on SSH (SSH_CONNECTION and "" or "unnamedplus") so
-- Neovim auto-enables OSC 52 -- but that branch is unreachable on a macOS
-- remote, where has('mac') selects the remote `pbcopy` first (see
-- $VIMRUNTIME/autoload/provider/clipboard.vim). Opt in to OSC 52 explicitly and
-- restore unnamedplus so a plain `y` still copies.
if vim.env.SSH_TTY or vim.env.SSH_CONNECTION then
  local osc52 = require("vim.ui.clipboard.osc52")
  -- Paste from the unnamed register rather than querying the terminal: an
  -- OSC 52 read round-trips and can hang for up to 10s over SSH + tmux.
  local paste = function()
    return { vim.split(vim.fn.getreg('"'), "\n"), vim.fn.getregtype('"') }
  end
  o.clipboard = "unnamedplus"
  g.clipboard = {
    name = "OSC 52",
    copy = { ["+"] = osc52.copy("+"), ["*"] = osc52.copy("*") },
    paste = { ["+"] = paste, ["*"] = paste },
  }
end

-- ui
g.gui_font_fize = 16
g.gui_font_face = "Iosevka NF"
o.guifont = "Iosevka NF:h16"
o.guifont = string.format("%s:h%s", g.gui_font_face, g.gui_font_size)

-- cursor
o.guicursor = "n-v-c-sm:block,i-ci-ve:ver25,r-cr-o:hor20"

-- line wrapping
o.whichwrap:append("<>[]hl")
o.autoindent = true

-- backspace
o.backspace = { "start", "eol", "indent" }
o.breakindent = true

-- split windows
o.splitright = true
o.splitbelow = true

-- add a ruler
o.colorcolumn = "80,120"

-- show whitespace characters
local space = "·"

o.list = true
o.listchars:append({
  tab = ">—",
  multispace = space,
  extends = ">",
  precedes = "<",
  lead = space,
  trail = space,
})

-- disable native bufferline
o.showtabline = 0

-- command options
o.laststatus = 3

-- spell checking
o.spell = true

-- backspace
o.backspace = { "start", "eol", "indent" }
o.breakindent = true

-- smooth scrolling
o.smoothscroll = false

-- disable lsp logs -- this will grow infinitely so only enable it if you need it
vim.lsp.log.set_level("error")

-- use ty for lsp
vim.g.lazyvim_python_lsp = "ty"
