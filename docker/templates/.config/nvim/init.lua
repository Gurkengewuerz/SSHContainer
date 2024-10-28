-- Learn about Neovim's lua api
-- https://neovim.io/doc/user/lua-guide.html

require("config.lazy")

local function paste()
    return {
        vim.split(vim.fn.getreg(''), '\n'),
        vim.fn.getregtype(''),
    }
end

if vim.env.SSH_TTY then
    vim.g.clipboard = {
        name = 'OSC 52',
        copy = {
            ['+'] = require('vim.ui.clipboard.osc52').copy('+'),
            ['*'] = require('vim.ui.clipboard.osc52').copy('*'),
        },
        paste = {
            ['+'] = paste,
            ['*'] = paste,
        },
    }
end
