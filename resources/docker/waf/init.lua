local common = dofile("/usr/local/share/nginx-ui/waf/common.lua")

if not common.enabled() then
    return
end

require "resty.core"

local lua_resty_waf = require "resty.waf"
lua_resty_waf.init()

ngx.log(ngx.NOTICE, "lua-resty-waf initialized in ", common.mode(), " mode")
