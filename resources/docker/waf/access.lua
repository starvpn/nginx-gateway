local common = dofile("/usr/local/share/nginx-ui/waf/common.lua")

if not common.enabled() then
    return
end

local waf = common.new_waf()
waf:exec()
