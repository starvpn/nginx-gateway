package settings

const DefaultWAFConfigPath = "/etc/nginx/waf/settings.lua"

// WAF contains global lua-resty-waf runtime controls.
// The first productized version is intentionally global and defaults to disabled.
type WAF struct {
	Enabled             bool   `json:"enabled"`
	Mode                string `json:"mode" binding:"omitempty,oneof=SIMULATE ACTIVE INACTIVE"`
	ScoreThreshold      int    `json:"score_threshold" binding:"omitempty,min=1,max=100"`
	DenyStatus          int    `json:"deny_status" binding:"omitempty,min=400,max=599"`
	Debug               bool   `json:"debug"`
	EventLogAlteredOnly bool   `json:"event_log_altered_only"`
	ConfigPath          string `json:"config_path" protected:"true"`
}

var WAFSettings = &WAF{
	Enabled:             false,
	Mode:                "SIMULATE",
	ScoreThreshold:      5,
	DenyStatus:          403,
	Debug:               false,
	EventLogAlteredOnly: true,
	ConfigPath:          DefaultWAFConfigPath,
}

func (w WAF) WithDefaults() WAF {
	if w.Mode != "ACTIVE" && w.Mode != "SIMULATE" && w.Mode != "INACTIVE" {
		w.Mode = "SIMULATE"
	}
	if w.ScoreThreshold <= 0 {
		w.ScoreThreshold = 5
	}
	if w.ScoreThreshold > 100 {
		w.ScoreThreshold = 100
	}
	if w.DenyStatus < 400 || w.DenyStatus > 599 {
		w.DenyStatus = 403
	}
	if w.ConfigPath == "" {
		w.ConfigPath = DefaultWAFConfigPath
	}
	return w
}

func (w *WAF) ApplyDefaults() {
	if w == nil {
		return
	}
	*w = w.WithDefaults()
}
