package nginx

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
)

func TestExtractConfigureArg(t *testing.T) {
	t.Parallel()

	output := `
nginx version: nginx/1.25.2
configure arguments: --prefix="/Program Files/Nginx" --conf-path='/Program Files/Nginx/conf/nginx.conf' --pid-path=/var/run/nginx.pid
`

	tests := []struct {
		name string
		flag string
		want string
	}{
		{
			name: "double quoted conf path",
			flag: "--conf-path",
			want: "/Program Files/Nginx/conf/nginx.conf",
		},
		{
			name: "single quoted conf path alias",
			flag: "conf-path",
			want: "/Program Files/Nginx/conf/nginx.conf",
		},
		{
			name: "unquoted pid path",
			flag: "pid-path",
			want: "/var/run/nginx.pid",
		},
		{
			name: "missing flag",
			flag: "--http-log-path",
			want: "",
		},
		{
			name: "prefix parsing",
			flag: "prefix",
			want: "/Program Files/Nginx",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := extractConfigureArg(output, tt.flag); got != tt.want {
				t.Fatalf("extractConfigureArg(%q) = %q, want %q", tt.flag, got, tt.want)
			}
		})
	}
}

func TestOpenRestyRelativeConfigurePathsUsePrefix(t *testing.T) {
	originalConfigDir := settings.NginxSettings.ConfigDir
	originalConfigPath := settings.NginxSettings.ConfigPath
	originalPIDPath := settings.NginxSettings.PIDPath
	originalAccessLogPath := settings.NginxSettings.AccessLogPath
	originalErrorLogPath := settings.NginxSettings.ErrorLogPath
	originalNginxVOutput := nginxVOutput
	originalNginxTOutput := nginxTOutput
	originalNginxPrefix := nginxPrefix

	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
		settings.NginxSettings.ConfigPath = originalConfigPath
		settings.NginxSettings.PIDPath = originalPIDPath
		settings.NginxSettings.AccessLogPath = originalAccessLogPath
		settings.NginxSettings.ErrorLogPath = originalErrorLogPath
		nginxVOutput = originalNginxVOutput
		nginxTOutput = originalNginxTOutput
		nginxPrefix = originalNginxPrefix
	})

	settings.NginxSettings.ConfigDir = ""
	settings.NginxSettings.ConfigPath = ""
	settings.NginxSettings.PIDPath = ""
	settings.NginxSettings.AccessLogPath = ""
	settings.NginxSettings.ErrorLogPath = ""
	nginxPrefix = ""
	nginxTOutput = ""

	prefix := "/usr/local/openresty/nginx"
	nginxVOutput = fmt.Sprintf(`
nginx version: openresty/1.27.1.2
configure arguments: --prefix=%s --conf-path=conf/nginx.conf --pid-path=logs/nginx.pid --http-log-path=logs/access.log --error-log-path=logs/error.log --modules-path=modules
`, prefix)

	if got := GetConfPath(); got != filepath.Join(prefix, "conf") {
		t.Fatalf("GetConfPath() = %q, want %q", got, filepath.Join(prefix, "conf"))
	}
	if got := GetConfEntryPath(); got != filepath.Join(prefix, "conf/nginx.conf") {
		t.Fatalf("GetConfEntryPath() = %q, want %q", got, filepath.Join(prefix, "conf/nginx.conf"))
	}
	if got := GetPIDPath(); got != filepath.Join(prefix, "logs/nginx.pid") {
		t.Fatalf("GetPIDPath() = %q, want %q", got, filepath.Join(prefix, "logs/nginx.pid"))
	}
	if got := GetAccessLogPath(); got != filepath.Join(prefix, "logs/access.log") {
		t.Fatalf("GetAccessLogPath() = %q, want %q", got, filepath.Join(prefix, "logs/access.log"))
	}
	if got := GetErrorLogPath(); got != filepath.Join(prefix, "logs/error.log") {
		t.Fatalf("GetErrorLogPath() = %q, want %q", got, filepath.Join(prefix, "logs/error.log"))
	}
	if got := GetModulesPath(); got != filepath.Join(prefix, "modules") {
		t.Fatalf("GetModulesPath() = %q, want %q", got, filepath.Join(prefix, "modules"))
	}
}

func TestGetConfAndPidPathsHandleSpaces(t *testing.T) {
	originalConfigDir := settings.NginxSettings.ConfigDir
	originalConfigPath := settings.NginxSettings.ConfigPath
	originalPIDPath := settings.NginxSettings.PIDPath
	originalNginxVOutput := nginxVOutput

	t.Cleanup(func() {
		settings.NginxSettings.ConfigDir = originalConfigDir
		settings.NginxSettings.ConfigPath = originalConfigPath
		settings.NginxSettings.PIDPath = originalPIDPath
		nginxVOutput = originalNginxVOutput
	})

	settings.NginxSettings.ConfigDir = ""
	settings.NginxSettings.ConfigPath = ""
	settings.NginxSettings.PIDPath = ""

	sampleConf := "/Program Files/nginx/conf/nginx.conf"
	samplePID := "/Program Files/nginx/logs/nginx.pid"

	nginxVOutput = fmt.Sprintf(`
nginx version: nginx/1.25.2
configure arguments: --conf-path="%s" --pid-path="%s"
`, sampleConf, samplePID)

	confDir := GetConfPath()
	expectedConfDir := filepath.Dir(sampleConf)
	if confDir != expectedConfDir {
		t.Fatalf("GetConfPath() = %q, want %q", confDir, expectedConfDir)
	}

	confEntry := GetConfEntryPath()
	if confEntry != sampleConf {
		t.Fatalf("GetConfEntryPath() = %q, want %q", confEntry, sampleConf)
	}

	pidPath := GetPIDPath()
	if pidPath != samplePID {
		t.Fatalf("GetPIDPath() = %q, want %q", pidPath, samplePID)
	}
}
