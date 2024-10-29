-- Learn about Neovim's lua api
-- https://neovim.io/doc/user/lua-guide.html

require("config.lazy")

local function paste()
	return {
		vim.split(vim.fn.getreg(""), "\n"),
		vim.fn.getregtype(""),
	}
end
