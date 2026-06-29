package nginx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readProjectFile(t *testing.T, elems ...string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(append([]string{"..", ".."}, elems...)...))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	return string(content)
}

func TestOfficialDockerConfig_PreservesForwardedHost(t *testing.T) {
	config := readProjectFile(t, "resources", "docker", "nginx-ui.conf")

	if !strings.Contains(config, "listen       80;") {
		t.Fatalf("official docker config = %q, want container port 80 listener", config)
	}

	if !strings.Contains(config, "proxy_pass http://127.0.0.1:9000/;") {
		t.Fatalf("official docker config = %q, want backend proxy to port 9000", config)
	}

	if !strings.Contains(config, "map $http_x_forwarded_host $forwarded_host") {
		t.Fatalf("official docker config = %q, want forwarded host fallback map", config)
	}

	if !strings.Contains(config, "proxy_set_header Host $forwarded_host;") {
		t.Fatalf("official docker config = %q, want Host header to preserve forwarded host", config)
	}

	if !strings.Contains(config, "proxy_set_header   X-Forwarded-Host     $forwarded_host;") {
		t.Fatalf("official docker config = %q, want X-Forwarded-Host header preservation", config)
	}
}

func TestOfficialDockerfileBundlesLuaRestyWaf(t *testing.T) {
	dockerfile := readProjectFile(t, "Dockerfile")

	requiredSnippets := []string{
		"ARG LUA_RESTY_WAF_REF=",
		"FROM ${OPENRESTY_IMAGE} AS lua-resty-waf-builder",
		"openresty-opm",
		"git clone --recurse-submodules",
		"p0pr0ck5/lua-resty-waf.git",
		"CFLAGS = -msse2 -msse3 -msse4.1 -O3/CFLAGS = -O3",
		"ROCK_DEPS  = \"lrexlib-pcre 2.7.2-1\" busted luafilesystem/ROCK_DEPS  = \"lrexlib-pcre 2.7.2-1\"",
		"rules/42000_xss.json",
		"bimport",
		"COPY --from=lua-resty-waf-builder /usr/local/openresty/site/ /usr/local/openresty/site/",
		"COPY --from=lua-resty-waf-builder /usr/local/lib/lua/5.1/rex_pcre.so /usr/local/openresty/site/lualib/rex_pcre.so",
		"COPY resources/docker/waf /usr/local/share/nginx-ui/waf",
		"RUN chmod -R a+rX /usr/local/share/nginx-ui/waf",
		"libpcre3",
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(dockerfile, snippet) {
			t.Fatalf("Dockerfile missing lua-resty-waf integration snippet %q", snippet)
		}
	}
}

func TestOfficialDockerfileSeedsLiveNginxConfig(t *testing.T) {
	dockerfile := readProjectFile(t, "Dockerfile")

	if !strings.Contains(dockerfile, "cp -a /usr/local/etc/nginx/. /etc/nginx/") {
		t.Fatalf("Dockerfile should seed /etc/nginx so the image works without a config volume")
	}
}

func TestDemoDockerfileBundlesLuaRestyWaf(t *testing.T) {
	dockerfile := readProjectFile(t, "demo.Dockerfile")

	requiredSnippets := []string{
		"ARG LUA_RESTY_WAF_REF=",
		"FROM ${OPENRESTY_IMAGE} AS lua-resty-waf-builder",
		"COPY --from=lua-resty-waf-builder /usr/local/openresty/site/ /usr/local/openresty/site/",
		"COPY --from=lua-resty-waf-builder /usr/local/lib/lua/5.1/rex_pcre.so /usr/local/openresty/site/lualib/rex_pcre.so",
		"COPY resources/docker/waf /usr/local/share/nginx-ui/waf",
		"RUN chmod -R a+rX /usr/local/share/nginx-ui/waf",
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(dockerfile, snippet) {
			t.Fatalf("demo.Dockerfile missing lua-resty-waf integration snippet %q", snippet)
		}
	}
}

func TestOfficialDockerNginxConfEnablesWafHooks(t *testing.T) {
	config := readProjectFile(t, "resources", "docker", "nginx.conf")

	requiredSnippets := []string{
		"env NGINX_UI_WAF_ENABLED;",
		"env NGINX_UI_WAF_CONFIG_PATH;",
		"env NGINX_UI_WAF_MODE;",
		"lua_shared_dict lua_resty_waf_storage",
		"lua_package_path \"/usr/local/openresty/site/lualib/?.lua;/usr/local/openresty/site/lualib/?/init.lua;;\";",
		"lua_package_cpath \"/usr/local/openresty/site/lualib/?.so;;\";",
		"init_by_lua_file /usr/local/share/nginx-ui/waf/init.lua;",
		"access_by_lua_file /usr/local/share/nginx-ui/waf/access.lua;",
		"header_filter_by_lua_file /usr/local/share/nginx-ui/waf/header_filter.lua;",
		"body_filter_by_lua_file /usr/local/share/nginx-ui/waf/body_filter.lua;",
		"log_by_lua_file /usr/local/share/nginx-ui/waf/log.lua;",
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(config, snippet) {
			t.Fatalf("resources/docker/nginx.conf missing WAF hook snippet %q", snippet)
		}
	}
}

func TestWafRuntimeScriptsDefaultDisabled(t *testing.T) {
	common := readProjectFile(t, "resources", "docker", "waf", "common.lua")
	access := readProjectFile(t, "resources", "docker", "waf", "access.lua")

	for _, snippet := range []string{
		"/etc/nginx/waf/settings.lua",
		"NGINX_UI_WAF_ENABLED",
		"NGINX_UI_WAF_CONFIG_PATH",
		"return value == \"1\" or value == \"true\" or value == \"yes\" or value == \"on\"",
		"SIMULATE",
		"ACTIVE",
		"lua_resty_waf_storage",
		"event_log_target",
		"error",
	} {
		if !strings.Contains(common, snippet) {
			t.Fatalf("common.lua missing WAF option snippet %q", snippet)
		}
	}

	for _, snippet := range []string{
		"if not common.enabled() then",
		"waf:exec()",
	} {
		if !strings.Contains(access, snippet) {
			t.Fatalf("access.lua missing WAF execution snippet %q", snippet)
		}
	}
}
