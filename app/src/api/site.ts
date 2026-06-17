import type { CertificateInfo } from '@/api/cert'
import type { ModelBase } from '@/api/curd'
import type { Namespace } from '@/api/namespace'
import type { NgxConfig } from '@/api/ngx'
import type { ConfigStatus, PrivateKeyType } from '@/constants'
import { extendCurdApi, http, useCurdApi } from '@uozi-admin/request'

export type SiteStatus = ConfigStatus.Enabled | ConfigStatus.Disabled | ConfigStatus.Maintenance
export type SiteType = 'static' | 'reverse_proxy' | 'custom'

export interface ProxyTarget {
  host: string
  port: string
  type: string // "proxy_pass" or "upstream"
}

export interface Site extends ModelBase {
  modified_at: string
  path: string
  advanced: boolean
  type: SiteType
  primary_domain: string
  domains: string[]
  remark: string
  site_dir: string
  proxy_target: string
  enable_ssl: boolean
  enable_ipv6: boolean
  access_log: boolean
  error_log: boolean
  name: string
  filepath: string
  config: string
  auto_cert: boolean
  tokenized?: NgxConfig
  cert_info?: Record<number, CertificateInfo[]>
  namespace_id: number
  namespace?: Namespace
  sync_node_ids: number[]
  urls?: string[]
  proxy_targets?: ProxyTarget[]
  status: SiteStatus
  dns_domain_id?: number | null
  dns_record_id?: string | null
  dns_record_name?: string | null
  dns_record_type?: string | null
  dns_record_exists?: boolean | null
}

export interface CreateSiteRequest {
  name: string
  type: SiteType
  primary_domain: string
  domains: string[]
  remark: string
  site_dir: string
  index: string
  proxy_target: string
  enable_ssl: boolean
  enable_ipv6: boolean
  access_log: boolean
  error_log: boolean
  custom_content: string
  namespace_id: number
  sync_node_ids: number[]
  overwrite: boolean
  post_action: string
  dns_domain_id?: number | null
  dns_record_id?: string | null
  dns_record_name?: string | null
  dns_record_type?: string | null
}

export interface CreateSiteResponse {
  message: string
  name: string
  site: Site
  content: string
}

export interface AutoCertRequest {
  dns_credential_id: number | null
  challenge_method: string
  domains: string[]
  key_type: PrivateKeyType
  acme_user_id?: number
}

const baseUrl = '/sites'

const site = extendCurdApi(useCurdApi<Site>(baseUrl), {
  create: (data: CreateSiteRequest) => http.post<CreateSiteResponse>(baseUrl, data),
  enable: (name: string) => http.post(`${baseUrl}/${encodeURIComponent(name)}/enable`),
  disable: (name: string) => http.post(`${baseUrl}/${encodeURIComponent(name)}/disable`),
  batchEnable: (names: string[]) => http.post(`${baseUrl}/batch/enable`, { names }),
  batchDisable: (names: string[]) => http.post(`${baseUrl}/batch/disable`, { names }),
  rename: (oldName: string, newName: string) => http.post(`${baseUrl}/${encodeURIComponent(oldName)}/rename`, { new_name: newName }),
  get_default_template: () => http.get('default_site_template'),
  add_auto_cert: (domain: string, data: AutoCertRequest) => http.post(`auto_cert/${encodeURIComponent(domain)}`, data),
  remove_auto_cert: (domain: string) => http.delete(`auto_cert/${encodeURIComponent(domain)}`),
  duplicate: (name: string, data: { name: string }) => http.post(`${baseUrl}/${encodeURIComponent(name)}/duplicate`, data),
  advance_mode: (name: string, data: { advanced: boolean }) => http.post(`${baseUrl}/${encodeURIComponent(name)}/advance`, data),
  enableMaintenance: (name: string) => http.post(`${baseUrl}/${encodeURIComponent(name)}/maintenance`),
})

export default site
