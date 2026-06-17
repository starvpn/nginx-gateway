package model

type Site struct {
	Model
	Path            string     `json:"path" gorm:"uniqueIndex"`
	Advanced        bool       `json:"advanced"`
	Type            string     `json:"type"`
	PrimaryDomain   string     `json:"primary_domain"`
	Domains         []string   `json:"domains" gorm:"serializer:json"`
	Remark          string     `json:"remark"`
	SiteDir         string     `json:"site_dir"`
	ProxyTarget     string     `json:"proxy_target"`
	EnableSSL       bool       `json:"enable_ssl"`
	EnableIPv6      bool       `json:"enable_ipv6"`
	AccessLog       bool       `json:"access_log"`
	ErrorLog        bool       `json:"error_log"`
	NamespaceID     uint64     `json:"namespace_id"`
	Namespace       *Namespace `json:"namespace,omitempty"`
	SyncNodeIDs     []uint64   `json:"sync_node_ids" gorm:"serializer:json"`
	DNSDomainID     *int       `json:"dns_domain_id"`     // Linked DNS domain ID
	DNSRecordID     *string    `json:"dns_record_id"`     // Linked DNS record ID
	DNSRecordName   *string    `json:"dns_record_name"`   // Cached DNS record name
	DNSRecordType   *string    `json:"dns_record_type"`   // Cached DNS record type (A, AAAA, CNAME)
	DNSRecordExists *bool      `json:"dns_record_exists"` // Whether the DNS record still exists
}

// GetPath implements ConfigEntity interface
func (s *Site) GetPath() string {
	return s.Path
}

// GetNamespaceID implements ConfigEntity interface
func (s *Site) GetNamespaceID() uint64 {
	return s.NamespaceID
}

// GetNamespace implements ConfigEntity interface
func (s *Site) GetNamespace() *Namespace {
	return s.Namespace
}
