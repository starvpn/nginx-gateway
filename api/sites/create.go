package sites

import (
	"net/http"

	"github.com/0xJacky/Nginx-UI/internal/site"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy"
)

type createSiteRequest struct {
	Name          string   `json:"name"`
	Type          string   `json:"type" binding:"required"`
	PrimaryDomain string   `json:"primary_domain"`
	Domains       []string `json:"domains"`
	Remark        string   `json:"remark"`
	SiteDir       string   `json:"site_dir"`
	Index         string   `json:"index"`
	ProxyTarget   string   `json:"proxy_target"`
	EnableSSL     bool     `json:"enable_ssl"`
	EnableIPv6    bool     `json:"enable_ipv6"`
	AccessLog     bool     `json:"access_log"`
	ErrorLog      bool     `json:"error_log"`
	CustomContent string   `json:"custom_content"`
	NamespaceID   uint64   `json:"namespace_id"`
	SyncNodeIDs   []uint64 `json:"sync_node_ids"`
	Overwrite     bool     `json:"overwrite"`
	PostAction    string   `json:"post_action"`
	DNSDomainID   *int     `json:"dns_domain_id"`
	DNSRecordID   *string  `json:"dns_record_id"`
	DNSRecordName *string  `json:"dns_record_name"`
	DNSRecordType *string  `json:"dns_record_type"`
}

func CreateSite(c *gin.Context) {
	var json createSiteRequest
	if !cosy.BindAndValid(c, &json) {
		return
	}

	result, err := site.CreateFromBlueprint(site.Blueprint{
		Name:          json.Name,
		Type:          json.Type,
		PrimaryDomain: json.PrimaryDomain,
		Domains:       json.Domains,
		Remark:        json.Remark,
		SiteDir:       json.SiteDir,
		Index:         json.Index,
		ProxyTarget:   json.ProxyTarget,
		EnableSSL:     json.EnableSSL,
		EnableIPv6:    json.EnableIPv6,
		AccessLog:     json.AccessLog,
		ErrorLog:      json.ErrorLog,
		CustomContent: json.CustomContent,
		NamespaceID:   json.NamespaceID,
		SyncNodeIDs:   json.SyncNodeIDs,
		Overwrite:     json.Overwrite,
		PostAction:    json.PostAction,
	})
	if err != nil {
		cosy.ErrHandler(c, err)
		return
	}

	if json.DNSDomainID != nil || json.DNSRecordID != nil || json.DNSRecordName != nil || json.DNSRecordType != nil {
		result.Site.DNSDomainID = json.DNSDomainID
		result.Site.DNSRecordID = json.DNSRecordID
		result.Site.DNSRecordName = json.DNSRecordName
		result.Site.DNSRecordType = json.DNSRecordType

		if json.DNSDomainID != nil && json.DNSRecordID != nil {
			exists := checkDNSRecordExists(*json.DNSDomainID, *json.DNSRecordID)
			result.Site.DNSRecordExists = &exists
		}

		if err := cosy.UseDB(c).Save(result.Site).Error; err != nil {
			cosy.ErrHandler(c, err)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
		"name":    result.Name,
		"site":    result.Site,
		"content": result.Content,
	})
}
