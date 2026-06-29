<script setup lang="ts">
import type { Cert } from '@/api/cert'
import type { NgxDirective, NgxServer } from '@/api/ngx'
import acme_user from '@/api/acme_user'
import cert from '@/api/cert'
import { AutoCertState } from '@/constants'
import IssueCert from '@/views/site/site_edit/components/Cert/IssueCert.vue'
import { useServerDirectives } from '@/views/site/site_edit/composables/useServerDirectives'
import dayjs from 'dayjs'
import { useSiteEditorStore } from '../SiteEditor/store'

type HttpMode = 'keep' | 'redirect' | 'disabled'
type SSLOption = 'existing' | 'manual'

const editorStore = useSiteEditorStore()
const { name, ngxConfig, curServerIdx, curDirectivesMap } = storeToRefs(editorStore)

const {
  findDirective,
  findDirectives,
  upsertDirective,
  removeDirective,
  removeDirectiveWhere,
  getListenPort,
} = useServerDirectives()

const router = useRouter()
const sslOption = ref<SSLOption>('existing')
const certificates = ref<Cert[]>([])
const certificatesLoading = ref(false)
const acmeUsers = ref<{ id: number, name: string, email: string }[]>([])
const acmeUsersLoading = ref(false)
const selectedAcmeUserID = ref<number>()

const defaultSSLCiphers = 'ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305:ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:DHE-RSA-AES256-GCM-SHA384:DHE-RSA-AES128-GCM-SHA256:ECDHE-RSA-AES256-SHA384:ECDHE-RSA-AES128-SHA256:!aNULL:!eNULL:!EXPORT:!DSS:!DES:!RC4:!3DES:!MD5:!PSK:!KRB5:!SRP:!CAMELLIA:!SEED'

function cloneServer(server: NgxServer): NgxServer {
  return JSON.parse(JSON.stringify(server))
}

function isSSLListen(directive: NgxDirective) {
  const params = directive.params.split(/\s+/)
  return directive.directive === 'listen' && (params.includes('ssl') || params.includes('quic'))
}

function isQUICListen(directive: NgxDirective) {
  return directive.directive === 'listen' && directive.params.split(/\s+/).includes('quic')
}

function isHTTPListen(directive: NgxDirective) {
  return directive.directive === 'listen' && !isSSLListen(directive)
}

function isIPv6Listen(directive: NgxDirective) {
  return directive.params.trim().startsWith('[::]')
}

function serverHasAnyListen(server?: NgxServer) {
  return server?.directives?.some(item => item.directive === 'listen') ?? false
}

function serverHasSSLListen(server?: NgxServer) {
  return server?.directives?.some(isSSLListen) ?? false
}

function serverHandlesHTTP(server?: NgxServer) {
  if (!server)
    return false

  return server.directives?.some(isHTTPListen)
    || (!serverHasSSLListen(server) && !serverHasAnyListen(server))
}

function getTLSServerIndex() {
  return ngxConfig.value.servers?.findIndex(server => serverHasSSLListen(server)) ?? -1
}

function getHTTPOnlyServerIndex() {
  return ngxConfig.value.servers?.findIndex(server => serverHandlesHTTP(server) && !serverHasSSLListen(server)) ?? -1
}

function getHTTPServerIndex() {
  const httpOnlyIndex = getHTTPOnlyServerIndex()
  if (httpOnlyIndex >= 0)
    return httpOnlyIndex

  return ngxConfig.value.servers?.findIndex(server => serverHandlesHTTP(server)) ?? -1
}

function getTLSServer() {
  const index = getTLSServerIndex()
  return index >= 0 ? ngxConfig.value.servers[index] : undefined
}

function getHTTPServer() {
  const index = getHTTPServerIndex()
  return index >= 0 ? ngxConfig.value.servers[index] : undefined
}

function getHTTPOnlyServer() {
  const index = getHTTPOnlyServerIndex()
  return index >= 0 ? ngxConfig.value.servers[index] : undefined
}

function getServerDirectives(server?: NgxServer) {
  if (!server)
    return []

  if (!server.directives)
    server.directives = []

  return server.directives
}

async function loadCertificates() {
  certificatesLoading.value = true
  try {
    const result: Cert[] = []
    let page = 1

    while (true) {
      const response = await cert.getList({ page })
      result.push(...response.data)

      const perPage = response.pagination?.per_page
      if (!perPage || response.data.length < perPage)
        break

      page++
    }

    certificates.value = result
  }
  finally {
    certificatesLoading.value = false
  }
}

async function loadAcmeUsers() {
  acmeUsersLoading.value = true
  try {
    const result: { id: number, name: string, email: string }[] = []
    let page = 1

    while (true) {
      const response = await acme_user.getList({ page })
      result.push(...response.data)

      const perPage = response.pagination?.per_page
      if (!perPage || response.data.length < perPage)
        break

      page++
    }

    acmeUsers.value = result
  }
  finally {
    acmeUsersLoading.value = false
  }
}

function getCertificateDomains(certificate: Cert) {
  return certificate.domains?.length ? certificate.domains : [certificate.name].filter(Boolean)
}

function getServerNames(server?: NgxServer) {
  return server?.directives
    ?.find(item => item.directive === 'server_name')
    ?.params
    ?.split(/\s+/)
    .map(item => item.trim())
    .filter(item => item && item !== '_') ?? []
}

function getCertificateMatchScore(certificate: Cert) {
  const names = getServerNames(getTLSServer() ?? getHTTPServer())
  const domains = getCertificateDomains(certificate)
  let score = 0

  for (const name of names) {
    for (const domain of domains) {
      if (domain === name) {
        score = Math.max(score, 3)
        continue
      }

      if (!domain.startsWith('*.'))
        continue

      const suffix = domain.slice(1)
      const base = domain.slice(2)
      if (name.endsWith(suffix) && name.split('.').length === base.split('.').length + 1)
        score = Math.max(score, 2)
    }
  }

  return score
}

function getCertificateSearchText(certificate: Cert) {
  return [
    certificate.name,
    ...getCertificateDomains(certificate),
    certificate.certificate_info?.issuer_name,
    certificate.certificate_info?.not_after,
  ].filter(Boolean).join(' ')
}

function filterCertificateOption(input: string, option?: unknown) {
  const label = (option as { label?: string })?.label ?? ''
  return label.toLowerCase().includes(input.toLowerCase())
}

function getCertificateTypeText(certificate: Cert) {
  if (certificate.auto_cert === AutoCertState.Enable)
    return $gettext('Managed Certificate')
  if (certificate.auto_cert === AutoCertState.Sync)
    return $gettext('Sync Certificate')
  if (certificate.auto_cert === AutoCertState.SelfSigned)
    return $gettext('Self-signed Certificate')
  return $gettext('General Certificate')
}

function getCertificateTypeColor(certificate: Cert) {
  if (certificate.auto_cert === AutoCertState.Enable)
    return 'processing'
  if (certificate.auto_cert === AutoCertState.Sync)
    return 'success'
  if (certificate.auto_cert === AutoCertState.SelfSigned)
    return 'cyan'
  return 'purple'
}

function getCertificateIssuerText(certificate: Cert) {
  const issuer = certificate.certificate_info?.issuer_name
  if (!issuer)
    return getCertificateTypeText(certificate)

  if (issuer.includes("Let's Encrypt"))
    return "Let's Encrypt"

  return issuer
}

function getCertificateExpiry(certificate: Cert) {
  const date = certificate.certificate_info?.not_after
  return date ? dayjs(date).format('YYYY-MM-DD') : '-'
}

function getCertificateExpiryValue(certificate: Cert) {
  const date = certificate.certificate_info?.not_after
  return date ? dayjs(date).valueOf() : 0
}

function getCurrentCertificatePaths() {
  const directives = getServerDirectives(getTLSServer())
  return {
    certificate: findDirective('ssl_certificate', directives)?.params,
    key: findDirective('ssl_certificate_key', directives)?.params,
  }
}

function getCurrentCertificateID() {
  const paths = getCurrentCertificatePaths()
  return certificates.value.find(certificate => {
    return certificate.ssl_certificate_path === paths.certificate
      && certificate.ssl_certificate_key_path === paths.key
  })?.id
}

function applyCertificate(certificate: Cert) {
  const server = ensureTLSServer()
  const directives = getServerDirectives(server)
  removeDirective('ssl_certificate', directives)
  removeDirective('ssl_certificate_key', directives)

  const serverNameIndex = directives.findIndex(item => item.directive === 'server_name')
  const insertIndex = serverNameIndex >= 0 ? serverNameIndex + 1 : directives.length
  directives.splice(insertIndex, 0,
    { directive: 'ssl_certificate', params: certificate.ssl_certificate_path },
    { directive: 'ssl_certificate_key', params: certificate.ssl_certificate_key_path },
  )

  selectedAcmeUserID.value = certificate.acme_user_id ?? 0
}

function clearCertificate() {
  const directives = getServerDirectives(getTLSServer())
  removeDirective('ssl_certificate', directives)
  removeDirective('ssl_certificate_key', directives)
}

function findBestCertificate() {
  return certificates.value
    .filter(certificate => certificate.ssl_certificate_path && certificate.ssl_certificate_key_path)
    .map(certificate => ({ certificate, score: getCertificateMatchScore(certificate) }))
    .filter(item => item.score > 0)
    .sort((a, b) => {
      if (a.score !== b.score)
        return b.score - a.score

      return getCertificateExpiryValue(b.certificate) - getCertificateExpiryValue(a.certificate)
    })[0]?.certificate
}

function applyBestCertificateIfNeeded() {
  if (getCurrentCertificatePaths().certificate)
    return

  const certificate = findBestCertificate()
  if (certificate)
    applyCertificate(certificate)
}

function syncActiveTLSServer() {
  const index = getTLSServerIndex()
  if (index >= 0)
    curServerIdx.value = index
}

function ensureTLSServer() {
  if (!ngxConfig.value.servers)
    ngxConfig.value.servers = []

  const existingIndex = getTLSServerIndex()
  if (existingIndex >= 0) {
    curServerIdx.value = existingIndex
    return ngxConfig.value.servers[existingIndex]
  }

  const source = getHTTPServer() ?? ngxConfig.value.servers[0] ?? { directives: [], locations: [] }
  const tlsServer = cloneServer(source)
  const directives = getServerDirectives(tlsServer)

  for (let i = directives.length - 1; i >= 0; i--) {
    if (directives[i].directive === 'listen')
      directives.splice(i, 1)
  }

  directives.unshift({ directive: 'listen', params: '443 ssl' })

  if (isIPv6Enabled.value)
    directives.unshift({ directive: 'listen', params: '[::]:443 ssl' })

  ngxConfig.value.servers.push(tlsServer)
  curServerIdx.value = ngxConfig.value.servers.length - 1
  return tlsServer
}

function removeTLSDirectives(directives: NgxDirective[]) {
  for (let i = directives.length - 1; i >= 0; i--) {
    const item = directives[i]
    if (isSSLListen(item))
      directives.splice(i, 1)
    else if (['ssl_certificate', 'ssl_certificate_key', 'ssl_protocols', 'ssl_ciphers', 'http2', 'http3'].includes(item.directive))
      directives.splice(i, 1)
    else if (item.directive === 'add_header' && (item.params.includes('Strict-Transport-Security') || item.params.includes('Alt-Svc')))
      directives.splice(i, 1)
  }
}

function ensureHTTPListen(directives: NgxDirective[]) {
  if (!directives.some(isHTTPListen))
    directives.unshift({ directive: 'listen', params: '80' })

  if (isIPv6Enabled.value && !directives.some(item => isHTTPListen(item) && isIPv6Listen(item)))
    directives.unshift({ directive: 'listen', params: '[::]:80' })
}

function ensureHTTPOnlyServer() {
  if (!ngxConfig.value.servers)
    ngxConfig.value.servers = []

  const existingServer = getHTTPOnlyServer()
  if (existingServer) {
    ensureHTTPListen(getServerDirectives(existingServer))
    return existingServer
  }

  const source = getTLSServer() ?? getHTTPServer() ?? ngxConfig.value.servers[0] ?? { directives: [], locations: [] }
  const httpServer = cloneServer(source)
  const directives = getServerDirectives(httpServer)

  removeTLSDirectives(directives)
  ensureHTTPListen(directives)

  ngxConfig.value.servers.push(httpServer)
  return httpServer
}

function removeHTTPListenDirectives(server?: NgxServer) {
  const directives = getServerDirectives(server)
  for (let i = directives.length - 1; i >= 0; i--) {
    if (isHTTPListen(directives[i]))
      directives.splice(i, 1)
  }
}

function removeHTTPConfig() {
  const servers = ngxConfig.value.servers
  if (!servers)
    return

  for (let index = servers.length - 1; index >= 0; index--) {
    const server = servers[index]
    const hasSSLListen = serverHasSSLListen(server)

    if (!serverHandlesHTTP(server))
      continue

    const directives = getServerDirectives(server)
    removeDirective('return', directives)
    removeHTTPListenDirectives(server)

    if (!hasSSLListen && servers.length > 1)
      servers.splice(index, 1)
  }

  if (curServerIdx.value >= (ngxConfig.value.servers?.length ?? 0))
    curServerIdx.value = 0

  syncActiveTLSServer()
}

function removeHTTPSConfig() {
  for (let index = (ngxConfig.value.servers?.length ?? 0) - 1; index >= 0; index--) {
    const server = ngxConfig.value.servers[index]
    const hadSSLListen = serverHasSSLListen(server)
    const directives = getServerDirectives(server)

    for (let i = directives.length - 1; i >= 0; i--) {
      const item = directives[i]
      if (item.directive === 'listen' && item.params.includes('ssl'))
        directives.splice(i, 1)
      else if (isQUICListen(item))
        directives.splice(i, 1)
      else if (['ssl_certificate', 'ssl_certificate_key', 'ssl_protocols', 'ssl_ciphers', 'http2', 'http3'].includes(item.directive))
        directives.splice(i, 1)
      else if (item.directive === 'add_header' && (item.params.includes('Strict-Transport-Security') || item.params.includes('Alt-Svc')))
        directives.splice(i, 1)
    }

    const hasPlainListen = directives.some(item => item.directive === 'listen' && !item.params.includes('ssl'))
    if (hadSSLListen && !hasPlainListen && ngxConfig.value.servers.length > 1)
      ngxConfig.value.servers.splice(index, 1)
  }

  if (curServerIdx.value >= (ngxConfig.value.servers?.length ?? 0))
    curServerIdx.value = 0
}

function setTLSListen(port: string) {
  const server = ensureTLSServer()
  const directives = getServerDirectives(server)
  const normalizedPort = port || '443'
  const ipv4Listen = directives.find(item => item.directive === 'listen' && item.params.includes('ssl') && !isIPv6Listen(item))
  const ipv6Listen = directives.find(item => item.directive === 'listen' && item.params.includes('ssl') && isIPv6Listen(item))

  if (ipv4Listen)
    ipv4Listen.params = `${normalizedPort} ssl`
  else
    directives.unshift({ directive: 'listen', params: `${normalizedPort} ssl` })

  if (isIPv6Enabled.value) {
    if (ipv6Listen)
      ipv6Listen.params = `[::]:${normalizedPort} ssl`
    else
      directives.unshift({ directive: 'listen', params: `[::]:${normalizedPort} ssl` })
  }
  else if (ipv6Listen) {
    directives.splice(directives.indexOf(ipv6Listen), 1)
  }

  if (http3Enabled.value)
    ensureHTTP3Directives(normalizedPort, directives)
}

function getQUICListenParams(port: string, isIPv6 = false) {
  const listenPort = isIPv6 ? `[::]:${port}` : port
  return `${listenPort} quic reuseport`
}

function getLastListenIndex(directives: NgxDirective[]) {
  for (let i = directives.length - 1; i >= 0; i--) {
    if (directives[i].directive === 'listen')
      return i
  }

  return -1
}

function upsertQUICListen(directives: NgxDirective[], port: string, isIPv6 = false) {
  const params = getQUICListenParams(port, isIPv6)
  const current = directives.find(item => isQUICListen(item) && isIPv6Listen(item) === isIPv6)

  if (current) {
    current.params = params
    return
  }

  directives.splice(getLastListenIndex(directives) + 1, 0, {
    directive: 'listen',
    params,
  })
}

function setHTTP3AltSvc(enabled: boolean, port: string, directives: NgxDirective[]) {
  removeDirectiveWhere('add_header', item => item.params.includes('Alt-Svc'), directives)

  if (!enabled)
    return

  directives.push({
    directive: 'add_header',
    params: `Alt-Svc 'h3=":${port}"; ma=86400'`,
  })
}

function ensureHTTP3Directives(port: string, directives: NgxDirective[]) {
  const normalizedPort = port || '443'
  upsertDirective('http3', 'on', directives)
  upsertQUICListen(directives, normalizedPort)

  if (isIPv6Enabled.value)
    upsertQUICListen(directives, normalizedPort, true)
  else
    removeDirectiveWhere('listen', item => isQUICListen(item) && isIPv6Listen(item), directives)

  setHTTP3AltSvc(true, normalizedPort, directives)
}

function removeHTTP3Directives(directives: NgxDirective[]) {
  removeDirective('http3', directives)
  removeDirectiveWhere('listen', isQUICListen, directives)
  setHTTP3AltSvc(false, '', directives)
}

const httpsEnabled = computed({
  get() {
    return getTLSServerIndex() >= 0
  },
  set(value: boolean) {
    if (value) {
      ensureTLSServer()
      httpMode.value = 'redirect'
      tlsProtocols.value = ['TLSv1.3', 'TLSv1.2']
      sslCiphers.value = defaultSSLCiphers
      applyBestCertificateIfNeeded()
      return
    }
    removeHTTPSConfig()
  },
})

const httpsPort = computed({
  get() {
    const server = getTLSServer()
    const listen = server?.directives?.find(item => item.directive === 'listen' && item.params.includes('ssl') && !isIPv6Listen(item))
    return getListenPort(listen?.params) || '443'
  },
  set(value: string) {
    setTLSListen(value.trim() || '443')
  },
})

const isIPv6Enabled = computed(() => {
  return ngxConfig.value.servers?.some(server => server.directives?.some(item => item.directive === 'listen' && isIPv6Listen(item))) ?? false
})

const httpMode = computed<HttpMode>({
  get() {
    const httpServer = getHTTPServer()
    const directives = getServerDirectives(httpServer)
    if (!serverHandlesHTTP(httpServer))
      return 'disabled'

    const returnDirective = directives.find(item => item.directive === 'return')
    if (returnDirective?.params.includes('https://'))
      return 'redirect'

    return 'keep'
  },
  set(value: HttpMode) {
    if (value === 'disabled') {
      removeHTTPConfig()
      return
    }

    const httpServer = value === 'redirect' ? ensureHTTPOnlyServer() : getHTTPServer() ?? ensureHTTPOnlyServer()
    const directives = getServerDirectives(httpServer)

    ensureHTTPListen(directives)

    if (value === 'redirect') {
      ngxConfig.value.servers?.forEach(server => {
        if (server !== httpServer && serverHasSSLListen(server))
          removeHTTPListenDirectives(server)
      })
      upsertDirective('return', '301 https://$host$request_uri', directives)
    }
    else
      removeDirective('return', directives)
  },
})

const selectedCertificateID = computed<number | undefined>({
  get() {
    return getCurrentCertificateID()
  },
  set(value) {
    if (!value) {
      clearCertificate()
      return
    }

    const certificate = certificates.value.find(item => item.id === value)
    if (certificate)
      applyCertificate(certificate)
  },
})

const acmeUserOptions = computed(() => [{
  label: $gettext('Existing/Self-signed Certificate'),
  value: 0,
}, ...acmeUsers.value.map(user => ({
  label: user.name || user.email,
  value: user.id,
}))])

const certificateOptions = computed(() => {
  return certificates.value
    .filter(certificate => certificate.ssl_certificate_path && certificate.ssl_certificate_key_path)
    .filter(certificate => {
      if (selectedAcmeUserID.value === undefined)
        return true

      if (selectedAcmeUserID.value === 0)
        return !certificate.acme_user_id || certificate.id === selectedCertificateID.value

      return certificate.acme_user_id === selectedAcmeUserID.value || certificate.id === selectedCertificateID.value
    })
    .sort((a, b) => {
      const score = getCertificateMatchScore(b) - getCertificateMatchScore(a)
      if (score !== 0)
        return score

      return getCertificateExpiryValue(b) - getCertificateExpiryValue(a)
    })
})

const noServerName = computed(() => !curDirectivesMap.value.server_name?.length)

function getHSTSDirective() {
  return findDirectives('add_header', getServerDirectives(getTLSServer()))
    .find(item => item.params.includes('Strict-Transport-Security'))
}

function setHSTSDirective(enabled: boolean, includeSubdomains: boolean) {
  ensureTLSServer()
  const directives = getServerDirectives(getTLSServer())
  removeDirectiveWhere('add_header', item => item.params.includes('Strict-Transport-Security'), directives)

  if (!enabled)
    return

  const valueParts = ['max-age=31536000']
  if (includeSubdomains)
    valueParts.push('includeSubDomains')

  directives.push({
    directive: 'add_header',
    params: `Strict-Transport-Security "${valueParts.join('; ')}" always`,
  })
}

const hstsEnabled = computed({
  get() {
    return Boolean(getHSTSDirective())
  },
  set(value: boolean) {
    setHSTSDirective(value, hstsIncludeSubdomains.value)
  },
})

const hstsIncludeSubdomains = computed({
  get() {
    return getHSTSDirective()?.params.includes('includeSubDomains') ?? false
  },
  set(value: boolean) {
    if (hstsEnabled.value)
      setHSTSDirective(true, value)
  },
})

const http3Enabled = computed({
  get() {
    const directives = getServerDirectives(getTLSServer())
    return findDirective('http3', directives)?.params === 'on'
      || findDirectives('listen', directives).some(isQUICListen)
  },
  set(value: boolean) {
    ensureTLSServer()
    const directives = getServerDirectives(getTLSServer())
    if (value) {
      if (!tlsProtocols.value.includes('TLSv1.3'))
        tlsProtocols.value = ['TLSv1.3', ...tlsProtocols.value]

      ensureHTTP3Directives(httpsPort.value, directives)
    }
    else
      removeHTTP3Directives(directives)
  },
})

const protocolOptions = computed(() => [
  { label: 'TLS 1.3', value: 'TLSv1.3' },
  { label: 'TLS 1.2', value: 'TLSv1.2' },
  { label: $gettext('TLS 1.1 (insecure)'), value: 'TLSv1.1' },
  { label: $gettext('TLS 1.0 (insecure)'), value: 'TLSv1' },
])

const tlsProtocols = computed<string[]>({
  get() {
    const params = findDirective('ssl_protocols', getServerDirectives(getTLSServer()))?.params
    return params ? params.split(/\s+/).filter(Boolean) : ['TLSv1.3', 'TLSv1.2']
  },
  set(value: string[]) {
    ensureTLSServer()
    const directives = getServerDirectives(getTLSServer())

    const protocols = http3Enabled.value && !value.includes('TLSv1.3')
      ? ['TLSv1.3', ...value]
      : value

    if (protocols.length)
      upsertDirective('ssl_protocols', protocols.join(' '), directives)
    else
      removeDirective('ssl_protocols', directives)
  },
})

const sslCiphers = computed({
  get() {
    return findDirective('ssl_ciphers', getServerDirectives(getTLSServer()))?.params ?? defaultSSLCiphers
  },
  set(value: string) {
    ensureTLSServer()
    const directives = getServerDirectives(getTLSServer())
    if (value.trim())
      upsertDirective('ssl_ciphers', value.trim(), directives)
    else
      removeDirective('ssl_ciphers', directives)
  },
})

watch(httpsEnabled, enabled => {
  if (enabled)
    syncActiveTLSServer()
}, { immediate: true })

watch(certificates, () => {
  if (httpsEnabled.value)
    applyBestCertificateIfNeeded()
})

onMounted(() => {
  loadCertificates()
  loadAcmeUsers()
})
</script>

<template>
  <div class="https-settings">
    <AAlert
      type="warning"
      class="https-helper-alert mb-6"
    >
      <template #message>
        {{ $gettext('Notice: Do not use SSL certificates for illegal websites.\nIf HTTPS is unavailable after enabling, check whether port 443 is correctly opened in the security group.') }}
      </template>
    </AAlert>

    <AForm
      layout="horizontal"
      class="https-form"
      label-align="right"
      :colon="false"
      :label-col="{ style: { width: '128px' } }"
      :wrapper-col="{ flex: '1' }"
    >
      <AFormItem :label="$gettext('Enable HTTPS')">
        <ASwitch v-model:checked="httpsEnabled" />
      </AFormItem>

      <template v-if="httpsEnabled">
        <AFormItem :label="$gettext('HTTPS Port')">
          <span class="form-static-value">{{ httpsPort }}</span>
        </AFormItem>

        <div class="ip-website-warning">
          {{ $gettext('For IP-based websites, set this site as the default site to access it normally.') }}
        </div>

        <ADivider class="settings-divider" orientation="left">
          {{ $gettext('Certificate Settings') }}
        </ADivider>

        <AFormItem :label="$gettext('HTTP Option')" required>
          <ASelect v-model:value="httpMode" class="form-control">
            <ASelectOption value="redirect">
              {{ $gettext('Automatically redirect HTTP to HTTPS') }}
            </ASelectOption>
            <ASelectOption value="keep">
              {{ $gettext('HTTP can be accessed directly') }}
            </ASelectOption>
            <ASelectOption value="disabled">
              {{ $gettext('Disable HTTP') }}
            </ASelectOption>
          </ASelect>
        </AFormItem>

        <AFormItem label="HSTS">
          <ACheckbox v-model:checked="hstsEnabled">
            {{ $gettext('Enable') }}
          </ACheckbox>
          <div class="input-help">
            {{ $gettext('Enabling HSTS can improve website security.') }}
          </div>
        </AFormItem>

        <AFormItem :label="$gettext('HSTS Subdomains')">
          <ACheckbox v-model:checked="hstsIncludeSubdomains" :disabled="!hstsEnabled">
            {{ $gettext('Enable') }}
          </ACheckbox>
          <div class="input-help">
            {{ $gettext('After enabling, the HSTS policy will apply to all subdomains of the current domain.') }}
          </div>
        </AFormItem>

        <AFormItem label="HTTP3">
          <ACheckbox v-model:checked="http3Enabled">
            {{ $gettext('Enable') }}
          </ACheckbox>
          <div class="input-help">
            {{ $gettext('HTTP/3 is an upgraded version of HTTP/2, providing faster connections and better performance. Not all browsers support HTTP/3, so enabling it may cause some browsers to fail to access the site.') }}
          </div>
        </AFormItem>

        <AAlert
          v-if="noServerName"
          type="info"
          show-icon
          class="form-offset mb-4"
          :message="$gettext('server_name is required before applying an ACME certificate.')"
        />

        <AFormItem :label="$gettext('SSL Option')" required>
          <ASelect v-model:value="sslOption" class="form-control">
            <ASelectOption value="existing">
              {{ $gettext('Select existing certificate') }}
            </ASelectOption>
            <ASelectOption value="manual">
              {{ $gettext('Import certificate manually') }}
            </ASelectOption>
          </ASelect>
        </AFormItem>

        <template v-if="sslOption === 'existing'">
          <AFormItem :label="$gettext('ACME Account')" required>
            <ASelect
              v-model:value="selectedAcmeUserID"
              class="form-control"
              allow-clear
              show-search
              :placeholder="$gettext('Select ACME account')"
              :loading="acmeUsersLoading"
              :options="acmeUserOptions"
            />
          </AFormItem>

          <AFormItem :label="$gettext('Certificate')" required>
            <ASelect
              v-model:value="selectedCertificateID"
              class="certificate-select"
              allow-clear
              show-search
              option-label-prop="label"
              :placeholder="$gettext('Select certificate')"
              :loading="certificatesLoading"
              :filter-option="filterCertificateOption"
            >
              <ASelectOption
                v-for="certificate in certificateOptions"
                :key="certificate.id"
                :value="certificate.id"
                :label="getCertificateSearchText(certificate)"
              >
                <div class="certificate-option">
                  <div class="certificate-option__name">
                    {{ getCertificateDomains(certificate).join(', ') }}
                  </div>
                  <ATag :color="getCertificateTypeColor(certificate)">
                    {{ getCertificateIssuerText(certificate) }}
                  </ATag>
                  <ATag color="blue">
                    {{ getCertificateExpiry(certificate) }}
                  </ATag>
                </div>
              </ASelectOption>
            </ASelect>
            <div class="certificate-actions">
              <AButton size="small" type="link" :loading="certificatesLoading" @click="loadCertificates">
                {{ $gettext('Refresh') }}
              </AButton>
              <AButton size="small" type="link" @click="router.push('/certificates/import')">
                {{ $gettext('Import Certificate') }}
              </AButton>
            </div>
          </AFormItem>
        </template>

        <template v-else>
          <AAlert
            type="info"
            show-icon
            class="form-offset mb-4"
            :message="$gettext('Import the certificate first, then return here and select it from the certificate list.')"
          />
          <div class="form-offset">
            <AButton type="primary" @click="router.push('/certificates/import')">
              {{ $gettext('Import Certificate') }}
            </AButton>
          </div>
        </template>

        <div class="certificate-extra-actions">
          <IssueCert :config-name="name" />
        </div>

        <ADivider class="settings-divider" orientation="left">
          {{ $gettext('SSL Protocol Settings') }}
        </ADivider>

        <AFormItem :label="$gettext('Supported Protocols')" required>
          <ACheckboxGroup v-model:value="tlsProtocols" :options="protocolOptions" />
        </AFormItem>

        <AFormItem :label="$gettext('Cipher Suite')" required>
          <ATextarea v-model:value="sslCiphers" class="cipher-textarea" :rows="4" />
        </AFormItem>
      </template>
    </AForm>
  </div>
</template>

<style scoped lang="less">
.https-settings {
  max-width: 1120px;
}

.https-form {
  max-width: 960px;
}

.https-helper-alert {
  max-width: 860px;

  :deep(.ant-alert-message) {
    white-space: pre-line;
  }
}

.form-control {
  width: 360px;
  max-width: 100%;
}

.certificate-select {
  width: 620px;
  max-width: 100%;
}

.cipher-textarea {
  width: 100%;
  max-width: 860px;
}

.form-static-value {
  display: inline-flex;
  min-height: 32px;
  align-items: center;
}

.ip-website-warning {
  margin: -2px 0 18px 128px;
  color: #fa8c16;
  font-size: 13px;
  line-height: 1.5;
}

.settings-divider {
  max-width: 860px;
  margin: 18px 0 24px;

  :deep(.ant-divider-inner-text) {
    font-weight: 600;
  }
}

.input-help {
  margin-top: 8px;
  color: #8c8c8c;
  font-size: 12px;
  line-height: 1.5;
}

.form-offset {
  margin-left: 128px;
}

.certificate-option {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
}

.certificate-option__name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.certificate-actions {
  display: flex;
  gap: 8px;
  margin-top: 8px;
}

.certificate-extra-actions {
  margin-bottom: 16px;
}

@media (max-width: 768px) {
  .https-form {
    :deep(.ant-form-item) {
      display: block;
    }

    :deep(.ant-form-item-label) {
      width: 100% !important;
      padding-bottom: 4px;
      text-align: left;
    }

    :deep(.ant-form-item-control) {
      max-width: 100%;
    }
  }

  .ip-website-warning,
  .form-offset {
    margin-left: 0;
  }

  .settings-divider,
  .https-helper-alert {
    max-width: 100%;
  }
}
</style>
