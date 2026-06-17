package site

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/pkg/errors"
)

const (
	TypeStatic       = "static"
	TypeReverseProxy = "reverse_proxy"
	TypeCustom       = "custom"
)

var domainLikePattern = regexp.MustCompile(`^[A-Za-z0-9*_.-]+$`)

type Blueprint struct {
	Name          string
	Type          string
	PrimaryDomain string
	Domains       []string
	Remark        string
	SiteDir       string
	Index         string
	ProxyTarget   string
	EnableSSL     bool
	EnableIPv6    bool
	AccessLog     bool
	ErrorLog      bool
	CustomContent string
	NamespaceID   uint64
	SyncNodeIDs   []uint64
	Overwrite     bool
	PostAction    string
}

type BlueprintResult struct {
	Name    string
	Content string
	Site    *model.Site
}

func CreateFromBlueprint(bp Blueprint) (*BlueprintResult, error) {
	normalized, err := normalizeBlueprint(bp)
	if err != nil {
		return nil, err
	}

	content, err := BuildConfigFromBlueprint(normalized)
	if err != nil {
		return nil, err
	}

	if err := Save(normalized.Name, content, normalized.Overwrite, normalized.NamespaceID, normalized.SyncNodeIDs, normalized.PostAction); err != nil {
		return nil, err
	}

	sitePath, err := ResolveAvailablePath(normalized.Name)
	if err != nil {
		return nil, err
	}

	siteModel := &model.Site{
		Path:          sitePath,
		Advanced:      normalized.Type == TypeCustom,
		Type:          normalized.Type,
		PrimaryDomain: normalized.PrimaryDomain,
		Domains:       normalized.Domains,
		Remark:        normalized.Remark,
		SiteDir:       normalized.SiteDir,
		ProxyTarget:   normalized.ProxyTarget,
		EnableSSL:     normalized.EnableSSL,
		EnableIPv6:    normalized.EnableIPv6,
		AccessLog:     normalized.AccessLog,
		ErrorLog:      normalized.ErrorLog,
		NamespaceID:   normalized.NamespaceID,
		SyncNodeIDs:   normalized.SyncNodeIDs,
	}

	if err := model.UseDB().Where("path = ?", sitePath).Assign(siteModel).FirstOrCreate(siteModel).Error; err != nil {
		return nil, err
	}

	return &BlueprintResult{
		Name:    normalized.Name,
		Content: content,
		Site:    siteModel,
	}, nil
}

func BuildConfigFromBlueprint(bp Blueprint) (string, error) {
	switch bp.Type {
	case TypeStatic:
		return buildStaticConfig(bp)
	case TypeReverseProxy:
		return buildReverseProxyConfig(bp)
	case TypeCustom:
		return nginx.FmtCode(bp.CustomContent)
	default:
		return "", fmt.Errorf("unsupported site type: %s", bp.Type)
	}
}

func normalizeBlueprint(bp Blueprint) (Blueprint, error) {
	bp.Type = strings.TrimSpace(bp.Type)
	if bp.Type == "" {
		bp.Type = TypeStatic
	}

	bp.PrimaryDomain = strings.TrimSpace(bp.PrimaryDomain)
	bp.Domains = normalizeDomains(bp.PrimaryDomain, bp.Domains)

	if bp.Type != TypeCustom && bp.PrimaryDomain == "" {
		return bp, errors.New("primary domain is required")
	}
	if bp.Type != TypeCustom && !isValidServerName(bp.PrimaryDomain) {
		return bp, fmt.Errorf("invalid primary domain: %s", bp.PrimaryDomain)
	}
	for _, domain := range bp.Domains {
		if !isValidServerName(domain) {
			return bp, fmt.Errorf("invalid domain: %s", domain)
		}
	}

	bp.Name = strings.TrimSpace(bp.Name)
	if bp.Name == "" {
		bp.Name = bp.PrimaryDomain
	}
	if bp.Name == "" {
		return bp, errors.New("site name is required")
	}

	if bp.Type == TypeStatic {
		bp.SiteDir = strings.TrimSpace(bp.SiteDir)
		if bp.SiteDir == "" {
			bp.SiteDir = path.Join("/var/www", bp.PrimaryDomain)
		}
		bp.Index = strings.TrimSpace(bp.Index)
		if bp.Index == "" {
			bp.Index = "index.html index.htm"
		}
	}

	if bp.Type == TypeReverseProxy {
		bp.ProxyTarget = strings.TrimSpace(bp.ProxyTarget)
		if bp.ProxyTarget == "" {
			return bp, errors.New("proxy target is required")
		}
		targetURL, err := url.Parse(bp.ProxyTarget)
		if err != nil {
			return bp, errors.Wrap(err, "invalid proxy target")
		}
		if targetURL.Scheme == "" || targetURL.Host == "" {
			return bp, errors.New("proxy target must include scheme and host")
		}
	}

	if bp.Type == TypeCustom && strings.TrimSpace(bp.CustomContent) == "" {
		return bp, errors.New("custom content is required")
	}

	return bp, nil
}

func normalizeDomains(primary string, domains []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(domains)+1)

	add := func(domain string) {
		domain = strings.TrimSpace(domain)
		if domain == "" {
			return
		}
		if _, ok := seen[domain]; ok {
			return
		}
		seen[domain] = struct{}{}
		result = append(result, domain)
	}

	add(primary)
	for _, domain := range domains {
		for _, item := range strings.FieldsFunc(domain, func(r rune) bool {
			return r == ',' || r == '\n' || r == '\t' || r == ' '
		}) {
			add(item)
		}
	}

	return result
}

func isValidServerName(name string) bool {
	if name == "_" || name == "localhost" {
		return true
	}
	return domainLikePattern.MatchString(name)
}

func buildStaticConfig(bp Blueprint) (string, error) {
	server := baseServer(bp)
	server.Directives = append(server.Directives,
		&nginx.NgxDirective{Directive: "root", Params: filepath.ToSlash(bp.SiteDir)},
		&nginx.NgxDirective{Directive: "index", Params: bp.Index},
	)
	server.Locations = append(server.Locations, &nginx.NgxLocation{
		Path: "/",
		Content: strings.Join([]string{
			"try_files $uri $uri/ =404;",
		}, "\n"),
	})

	return buildServerConfig(server)
}

func buildReverseProxyConfig(bp Blueprint) (string, error) {
	server := baseServer(bp)
	server.Locations = append(server.Locations, &nginx.NgxLocation{
		Path: "/",
		Content: strings.Join([]string{
			fmt.Sprintf("proxy_pass %s;", bp.ProxyTarget),
			"proxy_set_header Host $host;",
			"proxy_set_header X-Real-IP $remote_addr;",
			"proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;",
			"proxy_set_header X-Forwarded-Proto $scheme;",
			"proxy_http_version 1.1;",
			"proxy_set_header Upgrade $http_upgrade;",
			"proxy_set_header Connection \"upgrade\";",
		}, "\n"),
	})

	return buildServerConfig(server)
}

func baseServer(bp Blueprint) *nginx.NgxServer {
	directives := []*nginx.NgxDirective{
		{Directive: "listen", Params: "80"},
	}

	if bp.EnableIPv6 {
		directives = append(directives, &nginx.NgxDirective{Directive: "listen", Params: "[::]:80"})
	}

	if bp.EnableSSL {
		directives = append(directives, &nginx.NgxDirective{Directive: "listen", Params: "443 ssl"})
		if bp.EnableIPv6 {
			directives = append(directives, &nginx.NgxDirective{Directive: "listen", Params: "[::]:443 ssl"})
		}
	}

	directives = append(directives, &nginx.NgxDirective{
		Directive: "server_name",
		Params:    strings.Join(bp.Domains, " "),
	})

	if bp.AccessLog {
		directives = append(directives, &nginx.NgxDirective{
			Directive: "access_log",
			Params:    fmt.Sprintf("logs/%s.access.log", bp.Name),
		})
	}

	if bp.ErrorLog {
		directives = append(directives, &nginx.NgxDirective{
			Directive: "error_log",
			Params:    fmt.Sprintf("logs/%s.error.log", bp.Name),
		})
	}

	return &nginx.NgxServer{
		Directives: directives,
		Locations:  []*nginx.NgxLocation{},
	}
}

func buildServerConfig(server *nginx.NgxServer) (string, error) {
	ngxConfig := &nginx.NgxConfig{
		Servers: []*nginx.NgxServer{server},
	}
	return ngxConfig.BuildConfig()
}
