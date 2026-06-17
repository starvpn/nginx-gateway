package site

import (
	"strings"
	"testing"
)

func TestBuildConfigFromBlueprintStatic(t *testing.T) {
	content, err := BuildConfigFromBlueprint(Blueprint{
		Type:          TypeStatic,
		Name:          "example.com",
		PrimaryDomain: "example.com",
		Domains:       []string{"example.com", "www.example.com"},
		SiteDir:       "/var/www/example.com",
		Index:         "index.html index.htm",
		EnableIPv6:    true,
		AccessLog:     true,
		ErrorLog:      true,
	})
	if err != nil {
		t.Fatalf("BuildConfigFromBlueprint() error = %v", err)
	}

	assertContains(t, content, "server_name example.com www.example.com;")
	assertContains(t, content, "root /var/www/example.com;")
	assertContains(t, content, "try_files $uri $uri/ =404;")
	assertContains(t, content, "listen [::]:80;")
}

func TestBuildConfigFromBlueprintReverseProxy(t *testing.T) {
	content, err := BuildConfigFromBlueprint(Blueprint{
		Type:          TypeReverseProxy,
		Name:          "app.example.com",
		PrimaryDomain: "app.example.com",
		Domains:       []string{"app.example.com"},
		ProxyTarget:   "http://127.0.0.1:3000",
	})
	if err != nil {
		t.Fatalf("BuildConfigFromBlueprint() error = %v", err)
	}

	assertContains(t, content, "server_name app.example.com;")
	assertContains(t, content, "proxy_pass http://127.0.0.1:3000;")
	assertContains(t, content, "proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;")
}

func TestNormalizeBlueprintRequiresProxyTargetURL(t *testing.T) {
	_, err := normalizeBlueprint(Blueprint{
		Type:          TypeReverseProxy,
		Name:          "app.example.com",
		PrimaryDomain: "app.example.com",
		ProxyTarget:   "127.0.0.1:3000",
	})
	if err == nil {
		t.Fatal("normalizeBlueprint() error = nil, want invalid proxy target error")
	}
}

func assertContains(t *testing.T, content string, want string) {
	t.Helper()
	if !strings.Contains(content, want) {
		t.Fatalf("content does not contain %q:\n%s", want, content)
	}
}
