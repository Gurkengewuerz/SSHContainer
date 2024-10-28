vim.cmd.colorscheme('habamax')

-- Make line numbers default
vim.opt.number = true
-- You can also add relative line numbers, to help with jumping.
--  Experiment for yourself to see if you like it!
vim.opt.relativenumber = false

-- Clipboard
if vim.env.SSH_TTY then
    vim.opt.clipboard:append("unnamedplus")
    local function paste()
        return vim.split(vim.fn.getreg(""), "\n")
    end
    vim.g.clipboard = {
        name = "OSC 52",
        copy = {
            ["+"] = require("vim.ui.clipboard.osc52").copy("+"),
            ["*"] = require("vim.ui.clipboard.osc52").copy("*"),
        },
        paste = {
            ["+"] = paste,
            ["*"] = paste,
        },
    }
end

-- Case-insensitive searching UNLESS \C or one or more capital letters in the search term
vim.opt.ignorecase = true
vim.opt.smartcase = true

-- Keep signcolumn on by default
vim.opt.signcolumn = 'yes'

-- Set default tab options (but they should be overridden by sleuth)
vim.o.expandtab = true
vim.o.shiftwidth = 2
vim.o.softtabstop = 2
vim.o.shiftround = true
vim.o.smartindent = true
vim.o.tabstop = 2

-- Decrease update time
vim.opt.updatetime = 500

-- Decrease mapped sequence wait time
-- Displays which-key popup sooner
vim.opt.timeoutlen = 300

-- Show which line your cursor is on
vim.opt.cursorline = true
