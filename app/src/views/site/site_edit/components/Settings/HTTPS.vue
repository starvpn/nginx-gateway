<script setup lang="ts">
import type { Cert, CertificateInfo } from '@/api/cert'
import type { NgxDirective, NgxServer } from '@/api/ngx'
import CertInfo from '@/components/CertInfo/CertInfo.vue'
import ChangeCert from '@/views/site/site_edit/components/Cert/ChangeCert.vue'
import IssueCert from '@/views/site/site_edit/components/Cert/IssueCert.vue'
import SelfSignedCert from '@/views/site/site_edit/components/Cert/SelfSignedCert.vue'
import { useServerDirectives } from '@/views/site/site_edit/composables/useServerDirectives'
import { useSiteEditorStore } from '../SiteEditor/store'

type HttpMode = 'keep' | 'redirect' | 'disabled'

const editorStore = useSiteEditorStore()
const { name, ngxConfig, curServerIdx, curServerDirectives, curDirectivesMap, certInfoMap } = storeToRefs(editorStore)

const {
  findDirective,
  findDirectives,
  upsertDirective,
  removeDirective,
  removeDirectiveWhere,
  getListenPort,
} = useServerDirectives()

const changedCerts = ref<Cert[]>([])

function cloneServer(server: NgxServer): NgxServer {
  return JSON.parse(JSON.stringify(server))
}

function isSSLListen(directive: NgxDirective) {
  return directive.directive === 'listen' && directive.params.split(/\s+/).includes('ssl')
}

function isIPv6Listen(directive: NgxDirective) {
  return directive.params.trim().startsWith('[::]')
}

function serverHasSSLListen(server?: NgxServer) {
  return server?.directives?.some(isSSLListen) ?? false
}

function getTLSServerIndex() {
  return ngxConfig.value.servers?.findIndex(server => serverHasSSLListen(server)) ?? -1
}

function getHTTPServerIndex() {
  return ngxConfig.value.servers?.findIndex(server => !serverHasSSLListen(server)) ?? -1
}

function getTLSServer() {
  const index = getTLSServerIndex()
  return index >= 0 ? ngxConfig.value.servers[index] : undefined
}

function getHTTPServer() {
  const index = getHTTPServerIndex()
  return index >= 0 ? ngxConfig.value.servers[index] : undefined
}

function getServerDirectives(server?: NgxServer) {
  if (!server)
    return []

  if (!server.directives)
    server.directives = []

  return server.directives
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

  changedCerts.value = []
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
    const hasHTTPListen = directives.some(item => item.directive === 'listen' && !item.params.includes('ssl'))
    if (!hasHTTPListen)
      return 'disabled'

    const returnDirective = directives.find(item => item.directive === 'return')
    if (returnDirective?.params.includes('https://'))
      return 'redirect'

    return 'keep'
  },
  set(value: HttpMode) {
    const httpServer = getHTTPServer() ?? ngxConfig.value.servers?.[0]
    const directives = getServerDirectives(httpServer)

    if (value === 'disabled') {
      for (let i = directives.length - 1; i >= 0; i--) {
        if (directives[i].directive === 'listen' && !directives[i].params.includes('ssl'))
          directives.splice(i, 1)
      }
      removeDirective('return', directives)
      return
    }

    if (!directives.some(item => item.directive === 'listen' && !item.params.includes('ssl')))
      directives.unshift({ directive: 'listen', params: '80' })

    if (value === 'redirect')
      upsertDirective('return', '301 https://$host$request_uri', directives)
    else
      removeDirective('return', directives)
  },
})

const currentCertInfo = computed<CertificateInfo[]>(() => {
  const tlsIndex = getTLSServerIndex()
  return tlsIndex >= 0 ? certInfoMap.value?.[tlsIndex] ?? [] : []
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

function handleCertChange(certs: Cert[]) {
  ensureTLSServer()
  changedCerts.value = certs
  const directives = getServerDirectives(getTLSServer())
    .filter(item => item.directive !== 'ssl_certificate' && item.directive !== 'ssl_certificate_key')

  certs.forEach(cert => {
    directives.push({ directive: 'ssl_certificate', params: cert.ssl_certificate_path })
    directives.push({ directive: 'ssl_certificate_key', params: cert.ssl_certificate_key_path })
  })

  curServerDirectives.value = directives
}

watch(httpsEnabled, enabled => {
  if (enabled)
    syncActiveTLSServer()
}, { immediate: true })
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

        <ARow v-if="currentCertInfo.length" :gutter="[16, 16]" class="mb-4">
          <ACol v-for="(certInfo, index) in currentCertInfo" :key="index" :xs="24" :lg="12">
            <CertInfo :cert="certInfo" />
          </ACol>
        </ARow>
        <AEmpty v-else class="mb-4" :description="$gettext('No certificate configured')" />

        <template v-if="changedCerts.length">
          <h3>{{ $ngettext('Changed Certificate', 'Changed Certificates', changedCerts.length) }}</h3>
          <ARow :gutter="[16, 16]" class="mb-4">
            <ACol v-for="cert in changedCerts" :key="cert.id" :xs="24" :lg="12">
              <CertInfo :cert="cert.certificate_info" />
            </ACol>
          </ARow>
        </template>

        <ASpace wrap class="mb-4">
          <ChangeCert @change="handleCertChange" />
          <SelfSignedCert />
        </ASpace>

        <IssueCert :config-name="name" />

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
</style>
