<script setup lang="ts">
import type { Cert } from '@/api/cert'
import type { NgxDirective, NgxServer } from '@/api/ngx'
import acme_user from '@/api/acme_user'
import cert from '@/api/cert'
import { AutoCertState } from '@/constants'
import IssueCert from '@/views/site/site_edit/components/Cert/IssueCert.vue'
import SelfSignedCert from '@/views/site/site_edit/components/Cert/SelfSignedCert.vue'
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

function cloneServer(server: NgxServer): NgxServer {
  return JSON.parse(JSON.stringify(server))
}

function isSSLListen(directive: NgxDirective) {
  return directive.directive === 'listen' && directive.params.split(/\s+/).includes('ssl')
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

  selectedAcmeUserID.value = certificate.acme_user_id || undefined
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
    else if (['ssl_certificate', 'ssl_certificate_key', 'ssl_protocols', 'ssl_ciphers', 'http2'].includes(item.directive))
      directives.splice(i, 1)
    else if (item.directive === 'add_header' && item.params.includes('Strict-Transport-Security'))
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
      else if (['ssl_certificate', 'ssl_certificate_key', 'ssl_protocols', 'ssl_ciphers', 'http2'].includes(item.directive))
        directives.splice(i, 1)
      else if (item.directive === 'add_header' && item.params.includes('Strict-Transport-Security'))
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
}

const httpsEnabled = computed({
  get() {
    return getTLSServerIndex() >= 0
  },
  set(value: boolean) {
    if (value) {
      ensureTLSServer()
      tlsProtocols.value = ['TLSv1.3', 'TLSv1.2']
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

const acmeUserOptions = computed(() => acmeUsers.value.map(user => ({
  label: user.name || user.email,
  value: user.id,
})))

const certificateOptions = computed(() => {
  return certificates.value
    .filter(certificate => certificate.ssl_certificate_path && certificate.ssl_certificate_key_path)
    .filter(certificate => {
      if (!selectedAcmeUserID.value)
        return true

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

const http2Enabled = computed({
  get() {
    return findDirective('http2', getServerDirectives(getTLSServer()))?.params === 'on'
      || findDirectives('listen', getServerDirectives(getTLSServer())).some(item => item.params.includes('http2'))
  },
  set(value: boolean) {
    ensureTLSServer()
    const directives = getServerDirectives(getTLSServer())
    if (value)
      upsertDirective('http2', 'on', directives)
    else
      removeDirective('http2', directives)
  },
})

const protocolOptions = [
  { label: 'TLS 1.3', value: 'TLSv1.3' },
  { label: 'TLS 1.2', value: 'TLSv1.2' },
  { label: 'TLS 1.1 (insecure)', value: 'TLSv1.1' },
  { label: 'TLS 1.0 (insecure)', value: 'TLSv1' },
]

const tlsProtocols = computed<string[]>({
  get() {
    const params = findDirective('ssl_protocols', getServerDirectives(getTLSServer()))?.params
    return params ? params.split(/\s+/).filter(Boolean) : ['TLSv1.3', 'TLSv1.2']
  },
  set(value: string[]) {
    ensureTLSServer()
    const directives = getServerDirectives(getTLSServer())
    if (value.length)
      upsertDirective('ssl_protocols', value.join(' '), directives)
    else
      removeDirective('ssl_protocols', directives)
  },
})

const sslCiphers = computed({
  get() {
    return findDirective('ssl_ciphers', getServerDirectives(getTLSServer()))?.params ?? ''
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
      show-icon
      class="mb-6"
      :message="$gettext('Do not use SSL certificates for illegal websites. If HTTPS is enabled but unavailable, check whether port 443 is open in your firewall or security group.')"
    />

    <AForm layout="vertical" class="https-form">
      <AFormItem :label="$gettext('Enable HTTPS')">
        <ASwitch v-model:checked="httpsEnabled" />
      </AFormItem>

      <template v-if="httpsEnabled">
        <AFormItem :label="$gettext('HTTPS Port')">
          <AInput v-model:value="httpsPort" class="max-w-180px" placeholder="443" />
        </AFormItem>

        <AFormItem :label="$gettext('HTTP Option')">
          <ASelect v-model:value="httpMode" class="max-w-420px">
            <ASelectOption value="keep">
              {{ $gettext('Keep HTTP available') }}
            </ASelectOption>
            <ASelectOption value="redirect">
              {{ $gettext('Redirect HTTP to HTTPS') }}
            </ASelectOption>
            <ASelectOption value="disabled">
              {{ $gettext('Disable HTTP') }}
            </ASelectOption>
          </ASelect>
        </AFormItem>

        <ADivider orientation="left">
          {{ $gettext('Certificate Settings') }}
        </ADivider>

        <AAlert
          v-if="noServerName"
          type="info"
          show-icon
          class="mb-4"
          :message="$gettext('server_name is required before applying an ACME certificate.')"
        />

        <AFormItem :label="$gettext('SSL Option')" required>
          <ASelect v-model:value="sslOption" class="max-w-420px">
            <ASelectOption value="existing">
              {{ $gettext('Use existing certificate') }}
            </ASelectOption>
            <ASelectOption value="manual">
              {{ $gettext('Import certificate manually') }}
            </ASelectOption>
          </ASelect>
        </AFormItem>

        <template v-if="sslOption === 'existing'">
          <AFormItem :label="$gettext('ACME User')">
            <ASelect
              v-model:value="selectedAcmeUserID"
              class="max-w-420px"
              allow-clear
              show-search
              :placeholder="$gettext('All ACME users')"
              :loading="acmeUsersLoading"
              :options="acmeUserOptions"
            />
          </AFormItem>

          <AFormItem :label="$gettext('Certificate')" required>
            <ASelect
              v-model:value="selectedCertificateID"
              class="max-w-620px"
              allow-clear
              show-search
              option-label-prop="label"
              :placeholder="$gettext('Select an existing certificate')"
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
            class="mb-4"
            :message="$gettext('Import the certificate first, then return here and select it from the certificate list.')"
          />
          <AButton type="primary" @click="router.push('/certificates/import')">
            {{ $gettext('Import Certificate') }}
          </AButton>
        </template>

        <div class="certificate-extra-actions">
          <SelfSignedCert />
          <IssueCert :config-name="name" />
        </div>

        <ADivider orientation="left">
          {{ $gettext('SSL Protocol Settings') }}
        </ADivider>

        <AFormItem label="HSTS">
          <ACheckbox v-model:checked="hstsEnabled">
            {{ $gettext('Enable') }}
          </ACheckbox>
          <div class="text-gray-400 mt-2">
            {{ $gettext('Enabling HSTS can improve website security.') }}
          </div>
        </AFormItem>

        <AFormItem :label="$gettext('HSTS Subdomains')">
          <ACheckbox v-model:checked="hstsIncludeSubdomains" :disabled="!hstsEnabled">
            {{ $gettext('Enable') }}
          </ACheckbox>
        </AFormItem>

        <AFormItem label="HTTP/2">
          <ACheckbox v-model:checked="http2Enabled">
            {{ $gettext('Enable') }}
          </ACheckbox>
        </AFormItem>

        <AFormItem :label="$gettext('Supported Protocols')">
          <ACheckboxGroup v-model:value="tlsProtocols" :options="protocolOptions" />
        </AFormItem>

        <AFormItem :label="$gettext('Cipher Suite')">
          <ATextarea v-model:value="sslCiphers" :rows="4" />
        </AFormItem>
      </template>
    </AForm>
  </div>
</template>

<style scoped lang="less">
.https-settings {
  max-width: 960px;
}

.https-form {
  max-width: 860px;
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
</style>
