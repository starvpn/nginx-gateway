local _M = {}

local config_loaded = false
local config = {}
local config_path = os.getenv("NGINX_UI_WAF_CONFIG_PATH") or "/etc/nginx/waf/settings.lua"

local function normalize(value)
    return string.lower(value or "")
end

local function bool_env(name, default)
    local value = normalize(os.getenv(name))
    if value == "" then
        return default
    end
    return value == "1" or value == "true" or value == "yes" or value == "on"
end

local function int_env(name, default, min, max)
    local raw = os.getenv(name)
    local value = tonumber(raw)
    if value == nil then
        return default
    end
    value = math.floor(value)
    if min ~= nil and value < min then
        return min
    end
    if max ~= nil and value > max then
        return max
    end
    return value
end

local function load_config()
    if config_loaded then
        return config
    end

    config_loaded = true
    local file = io.open(config_path, "r")
    if file == nil then
        config = {}
        return config
    end
    file:close()

    local ok, loaded = pcall(dofile, config_path)
    if ok and type(loaded) == "table" then
        config = loaded
    else
        config = {}
        ngx.log(ngx.WARN, "Failed to load WAF settings from ", config_path)
    end

    return config
end

local function bool_option(key, env_name, default)
    local cfg = load_config()
    if cfg[key] ~= nil then
        return cfg[key] == true
    end
    return bool_env(env_name, default)
end

local function int_option(key, env_name, default, min, max)
    local cfg = load_config()
    if cfg[key] ~= nil then
        local value = tonumber(cfg[key])
        if value ~= nil then
            value = math.floor(value)
            if min ~= nil and value < min then
                return min
            end
            if max ~= nil and value > max then
                return max
            end
            return value
        end
    end
    return int_env(env_name, default, min, max)
end

function _M.enabled()
    return bool_option("enabled", "NGINX_UI_WAF_ENABLED", false)
end

function _M.mode()
    local cfg = load_config()
    local raw_mode = cfg.mode or os.getenv("NGINX_UI_WAF_MODE") or "SIMULATE"
    local mode = string.upper(raw_mode)
    if mode ~= "ACTIVE" and mode ~= "SIMULATE" and mode ~= "INACTIVE" then
        ngx.log(ngx.WARN, "Invalid WAF mode=", mode, "; falling back to SIMULATE")
        return "SIMULATE"
    end
    return mode
end

function _M.apply_options(waf)
    waf:set_option("mode", _M.mode())
    waf:set_option("storage_zone", "lua_resty_waf_storage")
    waf:set_option("event_log_target", "error")
    waf:set_option("event_log_altered_only", bool_option("event_log_altered_only", "NGINX_UI_WAF_EVENT_LOG_ALTERED_ONLY", true))
    waf:set_option("score_threshold", int_option("score_threshold", "NGINX_UI_WAF_SCORE_THRESHOLD", 5, 1, 100))
    waf:set_option("deny_status", int_option("deny_status", "NGINX_UI_WAF_DENY_STATUS", 403, 400, 599))
    waf:set_option("debug", bool_option("debug", "NGINX_UI_WAF_DEBUG", false))
end

function _M.new_waf()
    local lua_resty_waf = require "resty.waf"
    local waf = lua_resty_waf:new()
    _M.apply_options(waf)
    return waf
end

return _M
