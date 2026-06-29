<script setup lang="ts">
import type { NgxDirective, NgxLocation, NgxServer } from '@/api/ngx'
import type { BasicAuthUser, SiteStatus } from '@/api/site'
import { CopyOutlined, InfoCircleOutlined, SendOutlined } from '@ant-design/icons-vue'
import { StdSelector } from '@uozi-admin/curd'
import { useClipboard } from '@vueuse/core'
import configApi from '@/api/config'
import namespace from '@/api/namespace'
import siteApi from '@/api/site'
import { NgxUpstream } from '@/components/NgxConfigEditor'
import NodeSelector from '@/components/NodeSelector'
import { PortScannerCompact } from '@/components/PortScanner'
import SyncNodesPreview from '@/components/SyncNodesPreview'
import { formatDateTime } from '@/lib/helper'
import { useSettingsStore } from '@/pinia'
import namespaceColumns from '@/views/namespace/columns'
import SiteStatusSelect from '@/views/site/components/SiteStatusSelect.vue'
import ConfigName from '@/views/site/site_edit/components/ConfigName/ConfigName.vue'
import HttpsSettings from '@/views/site/site_edit/components/Settings/HTTPS.vue'
import { useSiteEditorStore } from '../SiteEditor/store'
import Chat from './Chat.vue'
import ConfigTemplatePanel from './ConfigTemplate.vue'
import DNS from './DNS.vue'

const settings = useSettingsStore()
const { message } = App.useApp()

const editorStore = useSiteEditorStore()
const { name, data, ngxConfig, curServerIdx, curServer, curServerDirectives, advanceMode } = storeToRefs(editorStore)

const activeModule = ref('domain')
const modulesWithoutServerSelector = ['https', 'load-balance', 'other']

interface DomainRow {
  key: string
  domain: string
  port: string
}

interface DomainForm {
  domain: string
  port: number
}

interface DirectoryRow {
  key: string
  name: string
  path: string
  description: string
}

interface AuthBasicUserForm {
  username: string
  password: string
  remark: string
}

const domainModalVisible = ref(false)
const domainForm = reactive<DomainForm>({
  domain: '',
  port: 80,
})
const { copy } = useClipboard()
const defaultDocumentPlaceholder = [
  'index.php',
  'index.html',
  'index.htm',
  'default.php',
  'default.htm',
  'default.html',
].join('\n')
const indexFilesInput = ref('')
const rateLimitPlan = ref('current')
const defaultAuthBasicUserFile = ref('')
const authBasicUsers = ref<BasicAuthUser[]>([])
const authBasicLoading = ref(false)
const authBasicSwitching = ref(false)
const authUserDrawerVisible = ref(false)
const authUserSubmitting = ref(false)
const authUserMode = ref<'create' | 'edit'>('create')
const authUserDrawerWidth = 'min(720px, calc(100vw - 32px))'
const authUserForm = reactive<AuthBasicUserForm>({
  username: '',
  password: '',
  remark: '',
})

const domainColumns = computed(() => [{
  title: $gettext('Domain'),
  dataIndex: 'domain',
  key: 'domain',
}, {
  title: $gettext('Port'),
  dataIndex: 'port',
  key: 'port',
  width: 180,
}, {
  title: $gettext('Actions'),
  dataIndex: 'actions',
  key: 'actions',
  width: 160,
}])

const directoryColumns = computed(() => [{
  title: $gettext('Directory'),
  dataIndex: 'name',
  key: 'name',
  width: 160,
}, {
  title: $gettext('Path'),
  dataIndex: 'path',
  key: 'path',
}, {
  title: $gettext('Description'),
  dataIndex: 'description',
  key: 'description',
  width: 260,
}, {
  title: $gettext('Actions'),
  dataIndex: 'actions',
  key: 'actions',
  width: 120,
}])

const authBasicColumns = computed(() => [{
  title: $gettext('Username'),
  dataIndex: 'username',
  key: 'username',
}, {
  title: $gettext('Remark'),
  dataIndex: 'remark',
  key: 'remark',
}, {
  title: $gettext('Actions'),
  dataIndex: 'actions',
  key: 'actions',
  width: 160,
  align: 'right' as const,
}])

const rateLimitPlanOptions = computed(() => [{
  label: $gettext('Current'),
  value: 'current',
}])

const realIpHeaderOptions = computed(() => [{
  label: 'X-Real-IP',
  value: 'X-Real-IP',
}, {
  label: 'X-Forwarded-For',
  value: 'X-Forwarded-For',
}, {
  label: 'proxy_protocol',
  value: 'proxy_protocol',
}])

const serverOptions = computed(() => {
  return (ngxConfig.value.servers ?? []).map((server, index) => {
    const serverName = server.directives?.find(item => item.directive === 'server_name')?.params
    const listens = server.directives?.filter(item => item.directive === 'listen').map(item => item.params).join(', ')
    const label = [
      $gettext('Server %{n}', { n: String(index + 1) }),
      serverName,
      listens ? `(${listens})` : '',
    ].filter(Boolean).join(' ')

    return {
      label,
      value: index,
    }
  })
})

const showServerSelector = computed(() => serverOptions.value.length > 1 && !modulesWithoutServerSelector.includes(activeModule.value))

function ensureServer(): NgxServer {
  if (!ngxConfig.value.servers)
    ngxConfig.value.servers = []

  if (!curServer.value) {
    ngxConfig.value.servers.push({
      directives: [],
      locations: [],
    })
    curServerIdx.value = 0
  }

  if (!curServer.value.directives)
    curServer.value.directives = []

  if (!curServer.value.locations)
    curServer.value.locations = []

  return curServer.value
}

function ensureDirectives() {
  const server = ensureServer()
  if (!server.directives)
    server.directives = []

  return server.directives
}

function findDirective(directive: string) {
  return curServerDirectives.value?.find(item => item.directive === directive)
}

function findDirectives(directive: string) {
  return curServerDirectives.value?.filter(item => item.directive === directive) ?? []
}

function listenHasSSL(params = '') {
  return params.split(/\s+/).includes('ssl')
}

function listenIsIPv6(params = '') {
  return params.trim().startsWith('[::]')
}

function hasIPv6Listen() {
  return findDirectives('listen').some(item => listenIsIPv6(item.params))
}

function upsertDirective(directive: string, params: string) {
  const current = findDirective(directive)
  if (current) {
    current.params = params
    return
  }

  ensureDirectives().push({
    directive,
    params,
  })
}

function removeDirective(directive: string) {
  const directives = ensureDirectives()
  for (let i = directives.length - 1; i >= 0; i--) {
    if (directives[i].directive === directive)
      directives.splice(i, 1)
  }
}

function getListenDirective(isSSL: boolean, isIPv6 = false) {
  return findDirectives('listen').find(item => {
    const params = item.params ?? ''
    return listenHasSSL(params) === isSSL && listenIsIPv6(params) === isIPv6
  })
}

function upsertListen(isSSL: boolean, params: string) {
  const current = getListenDirective(isSSL)
  if (current) {
    current.params = params
    return
  }

  ensureDirectives().unshift({
    directive: 'listen',
    params,
  })
}

function getListenPort(params = '') {
  const listenAddress = params.split(/\s+/).find(Boolean) ?? ''
  const addressPort = listenAddress.match(/:(\d+)$/)

  if (addressPort)
    return addressPort[1]

  const plainPort = listenAddress.match(/^(\d+)$/)
  return plainPort?.[1] ?? ''
}

function replaceListenPort(params: string, port: string) {
  const parts = params.split(/\s+/).filter(Boolean)
  if (!parts.length)
    return port

  if (parts[0].startsWith('[::]:'))
    parts[0] = `[::]:${port}`
  else if (parts[0].includes(':'))
    parts[0] = parts[0].replace(/:\d+$/, `:${port}`)
  else
    parts[0] = port

  return parts.join(' ')
}

function getServerNameList() {
  return (findDirective('server_name')?.params ?? '')
    .split(/\s+/)
    .map(item => item.trim())
    .filter(Boolean)
}

function setServerNameList(domains: string[]) {
  const uniqueDomains = [...new Set(domains.map(item => item.trim()).filter(Boolean))]

  if (uniqueDomains.length) {
    upsertDirective('server_name', uniqueDomains.join(' '))
    return
  }

  removeDirective('server_name')
}

function getPrimaryHTTPPort() {
  return getListenPort(getListenDirective(false)?.params)
    || getListenPort(getListenDirective(true)?.params)
    || '80'
}

function syncHTTPPort(port: string, sslEnabled: boolean) {
  const sslPort = getListenPort(getListenDirective(true)?.params)
  const directives = ensureDirectives()

  if (sslEnabled && sslPort === port) {
    for (let i = directives.length - 1; i >= 0; i--) {
      if (directives[i].directive === 'listen' && !listenHasSSL(directives[i].params))
        directives.splice(i, 1)
    }
    return
  }

  const current = getListenDirective(false)
  const ipv6Current = getListenDirective(false, true)

  if (current)
    current.params = replaceListenPort(current.params, port)
  else
    upsertListen(false, port)

  if (hasIPv6Listen()) {
    if (ipv6Current)
      ipv6Current.params = replaceListenPort(ipv6Current.params, port)
    else
      ensureDirectives().unshift({ directive: 'listen', params: `[::]:${port}` })
  }
}

function parseDomainInput(value: string) {
  return value
    .split(/[\s,]+/)
    .map(item => item.trim())
    .filter(Boolean)
}

function getMainLocation() {
  return curServer.value?.locations?.find(location => location.path === '/')
}

function ensureMainLocation(): NgxLocation {
  const server = ensureServer()
  if (!server.locations)
    server.locations = []

  let location = getMainLocation()
  if (!location) {
    location = {
      path: '/',
      content: '',
      comments: '',
    }
    server.locations.push(location)
  }

  return location
}

function updateProxyPass(value: string) {
  const location = ensureMainLocation()
  const line = value.trim() ? `proxy_pass ${value.trim()};` : ''
  const lines = location.content.split('\n')
  const proxyPassIndex = lines.findIndex(item => {
    const trimmed = item.trim()
    return trimmed.startsWith('proxy_pass ') && trimmed.endsWith(';')
  })

  if (proxyPassIndex >= 0) {
    if (line)
      lines[proxyPassIndex] = line
    else
      lines.splice(proxyPassIndex, 1)

    location.content = lines.join('\n').trim()
    return
  }

  if (line)
    location.content = [line, location.content].filter(Boolean).join('\n')
}

function getLogPath(directive: NgxDirective | undefined, fallback: string) {
  return directive?.params?.split(/\s+/)[0] || fallback
}

function getProxyPass(content = '') {
  const line = content.split('\n').find(item => {
    const trimmed = item.trim()
    return trimmed.startsWith('proxy_pass ') && trimmed.endsWith(';')
  })
  if (!line)
    return ''

  return line.trim().slice('proxy_pass '.length, -1).trim()
}

function removeDirectives(directive: string) {
  removeDirective(directive)
}

function setOptionalDirective(directive: string, params: string) {
  if (params.trim())
    upsertDirective(directive, params.trim())
  else
    removeDirective(directive)
}

function removeManagedBlock(content: string, start: string, end: string) {
  const startIndex = content.indexOf(start)
  const endIndex = content.indexOf(end)

  if (startIndex === -1 || endIndex === -1 || endIndex < startIndex)
    return content

  return [
    content.slice(0, startIndex).trimEnd(),
    content.slice(endIndex + end.length).trimStart(),
  ].filter(Boolean).join('\n')
}

function setManagedLocationBlock(start: string, end: string, block: string) {
  const location = ensureMainLocation()
  const nextContent = removeManagedBlock(location.content, start, end)
  location.content = [nextContent, `${start}\n${block.trim()}\n${end}`]
    .filter(Boolean)
    .join('\n')
}

function removeManagedLocationBlock(start: string, end: string) {
  const location = ensureMainLocation()
  location.content = removeManagedBlock(location.content, start, end)
}

function hasManagedLocationBlock(start: string, end: string) {
  const content = getMainLocation()?.content ?? ''
  return content.includes(start) && content.includes(end)
}

const corsBlockStart = '# NGINX UI CORS START'
const corsBlockEnd = '# NGINX UI CORS END'
const antiLeechBlockStart = '# NGINX UI ANTI-LEECH START'
const antiLeechBlockEnd = '# NGINX UI ANTI-LEECH END'

function normalizePositiveInteger(value: number | string | null | undefined) {
  const numberValue = Number(value)

  if (!Number.isFinite(numberValue) || numberValue <= 0)
    return null

  return Math.floor(numberValue)
}

function getLimitConnZone(params = '') {
  return params.split(/\s+/).find(Boolean) ?? ''
}

function findLimitConnDirective(zones: string[]) {
  return findDirectives('limit_conn').find(item => zones.includes(getLimitConnZone(item.params)))
}

function getLimitConnValue(zones: string[]) {
  const params = findLimitConnDirective(zones)?.params ?? ''
  const value = Number(params.split(/\s+/)[1])

  return Number.isFinite(value) && value > 0 ? value : null
}

function removeLimitConn(zones: string[]) {
  const directives = ensureDirectives()

  for (let i = directives.length - 1; i >= 0; i--) {
    if (directives[i].directive === 'limit_conn' && zones.includes(getLimitConnZone(directives[i].params)))
      directives.splice(i, 1)
  }
}

function setLimitConn(zones: string[], zone: string, value: number | string | null | undefined) {
  const normalizedValue = normalizePositiveInteger(value)

  if (!normalizedValue) {
    removeLimitConn(zones)
    return
  }

  const current = findLimitConnDirective(zones)

  if (current) {
    current.params = `${zone} ${normalizedValue}`
    return
  }

  ensureDirectives().push({
    directive: 'limit_conn',
    params: `${zone} ${normalizedValue}`,
  })
}

function getLimitRateKB() {
  const params = findDirective('limit_rate')?.params.trim() ?? ''
  const matches = params.match(/^(\d+(?:\.\d+)?)([kmg])?$/i)

  if (!matches)
    return null

  const value = Number(matches[1])
  const unit = matches[2]?.toLowerCase()

  if (!Number.isFinite(value) || value <= 0)
    return null

  if (unit === 'm')
    return Math.round(value * 1024)

  if (unit === 'g')
    return Math.round(value * 1024 * 1024)

  return Math.round(value)
}

function setLimitRateKB(value: number | string | null | undefined) {
  const normalizedValue = normalizePositiveInteger(value)

  if (normalizedValue)
    upsertDirective('limit_rate', `${normalizedValue}k`)
  else
    removeDirective('limit_rate')
}

function hasRateLimitDirectives() {
  return Boolean(findDirective('limit_rate') || findDirectives('limit_conn').length)
}

function ensureRateLimitDefaults() {
  if (!getLimitConnValue(['perserver']))
    setLimitConn(['perserver'], 'perserver', 300)

  if (!getLimitConnValue(['perip', 'addr']))
    setLimitConn(['perip', 'addr'], 'perip', 25)

  if (!getLimitRateKB())
    setLimitRateKB(512)
}

const rateLimitEnabled = computed({
  get() {
    return hasRateLimitDirectives()
  },
  set(value: boolean) {
    if (value) {
      ensureRateLimitDefaults()
      return
    }

    removeDirective('limit_conn')
    removeDirective('limit_rate')
  },
})

const concurrentLimit = computed({
  get() {
    return getLimitConnValue(['perserver']) ?? undefined
  },
  set(value: number | string | null | undefined) {
    setLimitConn(['perserver'], 'perserver', value)
  },
})

const singleIPLimit = computed({
  get() {
    return getLimitConnValue(['perip', 'addr']) ?? undefined
  },
  set(value: number | string | null | undefined) {
    setLimitConn(['perip', 'addr'], 'perip', value)
  },
})

const singleRequestRateLimit = computed({
  get() {
    return getLimitRateKB() ?? undefined
  },
  set(value: number | string | null | undefined) {
    setLimitRateKB(value)
  },
})

const authBasic = computed({
  get() {
    return findDirective('auth_basic')?.params?.replace(/^"|"$/g, '') ?? ''
  },
  set(value: string) {
    setOptionalDirective('auth_basic', value.trim() ? `"${value.trim()}"` : '')
  },
})

const authBasicUserFile = computed({
  get() {
    return findDirective('auth_basic_user_file')?.params ?? ''
  },
  set(value: string) {
    setOptionalDirective('auth_basic_user_file', value)
  },
})

const authBasicEnabled = computed(() => Boolean(findDirective('auth_basic') || findDirective('auth_basic_user_file')))

const authBasicUserFilePath = computed(() => authBasicUserFile.value.trim() || defaultAuthBasicUserFile.value)

const authUserDrawerTitle = computed(() => authUserMode.value === 'create' ? $gettext('Create') : $gettext('Edit'))

async function loadDefaultAuthBasicUserFile() {
  if (defaultAuthBasicUserFile.value)
    return

  try {
    const response = await configApi.get_base_path() as { base_path: string }
    const basePath = response.base_path?.replace(/\/+$/, '')
    if (basePath)
      defaultAuthBasicUserFile.value = `${basePath}/.htpasswd`
  }
  catch {
    defaultAuthBasicUserFile.value = ''
  }
}

async function loadAuthBasicUsers() {
  const path = authBasicUserFilePath.value
  if (!path) {
    authBasicUsers.value = []
    return
  }

  authBasicLoading.value = true
  try {
    const response = await siteApi.get_basic_auth(name.value, path)
    authBasicUsers.value = response.users ?? []
  }
  catch {
    authBasicUsers.value = []
  }
  finally {
    authBasicLoading.value = false
  }
}

async function ensureAuthBasicForFile() {
  if (!authBasicUserFilePath.value)
    await loadDefaultAuthBasicUserFile()

  const path = authBasicUserFilePath.value
  if (!path) {
    message.warning($gettext('Password file path is required'))
    return false
  }

  await siteApi.ensure_basic_auth_file(name.value, { path })

  if (!authBasic.value.trim())
    authBasic.value = 'Restricted'

  if (!authBasicUserFile.value.trim())
    authBasicUserFile.value = path

  return true
}

async function handleAuthBasicEnabledChange(checked: boolean | string | number) {
  if (!checked) {
    removeDirective('auth_basic')
    removeDirective('auth_basic_user_file')
    return
  }

  authBasicSwitching.value = true
  try {
    if (await ensureAuthBasicForFile())
      await loadAuthBasicUsers()
  }
  catch {
    // Request errors are displayed by the global request handler.
  }
  finally {
    authBasicSwitching.value = false
  }
}

function resetAuthUserForm() {
  authUserForm.username = ''
  authUserForm.password = ''
  authUserForm.remark = ''
}

async function openCreateAuthUserDrawer() {
  if (!authBasicUserFilePath.value)
    await loadDefaultAuthBasicUserFile()

  authUserMode.value = 'create'
  resetAuthUserForm()
  authUserDrawerVisible.value = true
}

function openEditAuthUserDrawer(user: BasicAuthUser) {
  authUserMode.value = 'edit'
  authUserForm.username = user.username
  authUserForm.password = ''
  authUserForm.remark = user.remark ?? ''
  authUserDrawerVisible.value = true
}

function fillRandomAuthPassword() {
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789!@#$%^&*'
  const values = new Uint32Array(16)
  crypto.getRandomValues(values)
  authUserForm.password = Array.from(values, value => chars[value % chars.length]).join('')
}

async function submitAuthUser() {
  const username = authUserForm.username.trim()
  const password = authUserForm.password
  const remark = authUserForm.remark.trim()

  if (!username || (authUserMode.value === 'create' && !password)) {
    message.warning($gettext('Please fill in all required fields'))
    return
  }

  authUserSubmitting.value = true
  try {
    if (!await ensureAuthBasicForFile())
      return

    const path = authBasicUserFilePath.value
    if (authUserMode.value === 'create') {
      await siteApi.create_basic_auth_user(name.value, {
        path,
        username,
        password,
        remark,
      })
    }
    else {
      await siteApi.update_basic_auth_user(name.value, username, {
        path,
        password: password || undefined,
        remark,
      })
    }

    message.success($gettext('Saved successfully'))
    authUserDrawerVisible.value = false
    await loadAuthBasicUsers()
  }
  catch {
    // Request errors are displayed by the global request handler.
  }
  finally {
    authUserSubmitting.value = false
  }
}

async function deleteAuthUser(username: string) {
  const path = authBasicUserFilePath.value
  if (!path) {
    message.warning($gettext('Password file path is required'))
    return
  }

  try {
    await siteApi.delete_basic_auth_user(name.value, username, path)
    message.success($gettext('Deleted successfully'))
    await loadAuthBasicUsers()
  }
  catch {
    // Request errors are displayed by the global request handler.
  }
}

onMounted(() => {
  loadDefaultAuthBasicUserFile()
})

watch([activeModule, authBasicUserFilePath], ([module]) => {
  if (module === 'auth')
    loadAuthBasicUsers()
}, { immediate: true })

const corsAllowOrigin = ref('*')
const corsAllowMethods = ref('GET, POST, PUT, PATCH, DELETE, OPTIONS')
const corsAllowHeaders = ref('Authorization, Content-Type, Accept, Origin, User-Agent')
const corsAllowCredentials = ref(false)
const corsPreflightQuickResponse = ref(true)

function buildCORSLocationBlock() {
  const block = [
    `add_header Access-Control-Allow-Origin "${corsAllowOrigin.value}" always;`,
    `add_header Access-Control-Allow-Methods "${corsAllowMethods.value}" always;`,
    `add_header Access-Control-Allow-Headers "${corsAllowHeaders.value}" always;`,
  ]

  if (corsAllowCredentials.value)
    block.push('add_header Access-Control-Allow-Credentials "true" always;')

  if (corsPreflightQuickResponse.value)
    block.push('if ($request_method = OPTIONS) { return 204; }')

  return block.join('\n')
}

const corsEnabled = computed({
  get() {
    return hasManagedLocationBlock(corsBlockStart, corsBlockEnd)
  },
  set(value: boolean) {
    if (!value) {
      removeManagedLocationBlock(corsBlockStart, corsBlockEnd)
      return
    }

    setManagedLocationBlock(corsBlockStart, corsBlockEnd, buildCORSLocationBlock())
  },
})

watch([corsAllowOrigin, corsAllowMethods, corsAllowHeaders, corsAllowCredentials, corsPreflightQuickResponse], () => {
  if (corsEnabled.value)
    corsEnabled.value = true
})

const realIpFrom = computed({
  get() {
    return findDirectives('set_real_ip_from').map(item => item.params).join('\n')
  },
  set(value: string) {
    removeDirectives('set_real_ip_from')
    value.split(/[\n,]+/).map(item => item.trim()).filter(Boolean).forEach(params => {
      ensureDirectives().push({ directive: 'set_real_ip_from', params })
    })
  },
})

const realIpHeader = computed({
  get() {
    return findDirective('real_ip_header')?.params ?? ''
  },
  set(value: string) {
    setOptionalDirective('real_ip_header', value)
  },
})

function ensureRealIpDefaults() {
  if (!realIpFrom.value.trim())
    realIpFrom.value = '127.0.0.1'

  if (!realIpHeader.value.trim())
    realIpHeader.value = 'X-Real-IP'
}

const realIpEnabled = computed({
  get() {
    return Boolean(findDirectives('set_real_ip_from').length
      || findDirective('real_ip_header')
      || findDirective('real_ip_recursive'))
  },
  set(value: boolean) {
    if (value) {
      ensureRealIpDefaults()
      return
    }

    removeDirectives('set_real_ip_from')
    removeDirective('real_ip_header')
    removeDirective('real_ip_recursive')
  },
})

const rewriteRules = computed({
  get() {
    return findDirectives('rewrite').map(item => item.params).join('\n')
  },
  set(value: string) {
    removeDirectives('rewrite')
    value.split('\n').map(item => item.trim()).filter(Boolean).forEach(params => {
      ensureDirectives().push({ directive: 'rewrite', params })
    })
  },
})

const antiLeechReferers = ref('none blocked server_names')

const antiLeechEnabled = computed({
  get() {
    return hasManagedLocationBlock(antiLeechBlockStart, antiLeechBlockEnd)
  },
  set(value: boolean) {
    if (!value) {
      removeManagedLocationBlock(antiLeechBlockStart, antiLeechBlockEnd)
      return
    }

    setManagedLocationBlock(antiLeechBlockStart, antiLeechBlockEnd, [
      `valid_referers ${antiLeechReferers.value};`,
      'if ($invalid_referer) { return 403; }',
    ].join('\n'))
  },
})

watch(antiLeechReferers, () => {
  if (antiLeechEnabled.value)
    antiLeechEnabled.value = true
})

const redirectReturn = computed({
  get() {
    return findDirective('return')?.params ?? ''
  },
  set(value: string) {
    setOptionalDirective('return', value)
  },
})

const domainRows = computed<DomainRow[]>(() => {
  const port = getPrimaryHTTPPort()

  return getServerNameList().map((domain, index) => ({
    key: `${domain}-${index}`,
    domain,
    port,
  }))
})

function openDomainModal() {
  domainForm.domain = ''
  domainForm.port = Number(getPrimaryHTTPPort()) || 80
  domainModalVisible.value = true
}

function handleAddDomain() {
  const domains = parseDomainInput(domainForm.domain)
  const port = String(domainForm.port || 80)

  if (!domains.length || Number(port) < 1 || Number(port) > 65535) {
    message.warning($gettext('Please fill in all required fields'))
    return
  }

  setServerNameList([...getServerNameList(), ...domains])
  syncHTTPPort(port, false)
  domainModalVisible.value = false
}

function deleteDomain(domain: string) {
  setServerNameList(getServerNameList().filter(item => item !== domain))
}

function normalizeDirectoryPath(path: string) {
  const trimmedPath = path.trim()

  if (trimmedPath === '/')
    return trimmedPath

  return trimmedPath.replace(/\/+$/, '')
}

function joinDirectoryPath(basePath: string, directoryName: string) {
  const normalizedBasePath = normalizeDirectoryPath(basePath)

  if (!normalizedBasePath)
    return directoryName

  if (normalizedBasePath === '/')
    return `/${directoryName}`

  return `${normalizedBasePath}/${directoryName}`
}

function getDirectoryName(path: string) {
  return normalizeDirectoryPath(path).split('/').filter(Boolean).pop() ?? ''
}

function getParentDirectory(path: string) {
  const normalizedPath = normalizeDirectoryPath(path)
  const lastSeparatorIndex = normalizedPath.lastIndexOf('/')

  if (lastSeparatorIndex <= 0)
    return normalizedPath

  return normalizedPath.slice(0, lastSeparatorIndex)
}

function normalizeIndexFiles(value: string) {
  return value
    .split(/[\s,]+/)
    .map(item => item.trim())
    .filter(Boolean)
}

function formatIndexFiles(value: string) {
  return normalizeIndexFiles(value).join('\n')
}

const siteRoot = computed({
  get() {
    return findDirective('root')?.params ?? ''
  },
  set(value: string) {
    if (value.trim())
      upsertDirective('root', value.trim())
    else
      removeDirective('root')
  },
})

const normalizedSiteRoot = computed(() => normalizeDirectoryPath(siteRoot.value))

const hasIndexRootDirectory = computed(() => getDirectoryName(normalizedSiteRoot.value) === 'index')

const siteWorkspace = computed(() => {
  if (!normalizedSiteRoot.value)
    return ''

  return hasIndexRootDirectory.value
    ? getParentDirectory(normalizedSiteRoot.value)
    : normalizedSiteRoot.value
})

const directoryRows = computed<DirectoryRow[]>(() => {
  if (!siteWorkspace.value)
    return []

  return [{
    key: 'ssl',
    name: 'ssl',
    path: joinDirectoryPath(siteWorkspace.value, 'ssl'),
    description: $gettext('Site certificates'),
  }, {
    key: 'log',
    name: 'log',
    path: joinDirectoryPath(siteWorkspace.value, 'log'),
    description: $gettext('Site logs'),
  }, {
    key: 'root',
    name: hasIndexRootDirectory.value ? 'index' : 'root',
    path: normalizedSiteRoot.value,
    description: $gettext('Configured NGINX root directory'),
  }]
})

async function copyDirectoryPath(path: string) {
  if (!path) {
    message.warning($gettext('Nothing to copy'))
    return
  }

  try {
    await copy(path)
    message.success($gettext('Path copied to clipboard'))
  }
  catch {
    message.error($gettext('Failed to copy to clipboard'))
  }
}

const indexFiles = computed({
  get() {
    return findDirective('index')?.params ?? ''
  },
  set(value: string) {
    const normalizedValue = normalizeIndexFiles(value).join(' ')

    if (normalizedValue)
      upsertDirective('index', normalizedValue)
    else
      removeDirective('index')
  },
})

watch(indexFiles, value => {
  indexFilesInput.value = formatIndexFiles(value)
}, { immediate: true })

function handleIndexFilesBlur() {
  indexFiles.value = indexFilesInput.value
  indexFilesInput.value = formatIndexFiles(indexFiles.value)
}

const proxyTarget = computed({
  get() {
    return getProxyPass(getMainLocation()?.content)
  },
  set(value: string) {
    updateProxyPass(value)
  },
})

const hasAccessLog = computed({
  get() {
    return Boolean(findDirective('access_log'))
  },
  set(value: boolean) {
    if (value)
      upsertDirective('access_log', `logs/${name.value}.access.log`)
    else
      removeDirective('access_log')
  },
})

const hasErrorLog = computed({
  get() {
    return Boolean(findDirective('error_log'))
  },
  set(value: boolean) {
    if (value)
      upsertDirective('error_log', `logs/${name.value}.error.log`)
    else
      removeDirective('error_log')
  },
})

const accessLogPath = computed({
  get() {
    return getLogPath(findDirective('access_log'), `logs/${name.value}.access.log`)
  },
  set(value: string) {
    if (value.trim())
      upsertDirective('access_log', value.trim())
  },
})

const errorLogPath = computed({
  get() {
    return getLogPath(findDirective('error_log'), `logs/${name.value}.error.log`)
  },
  set(value: string) {
    if (value.trim())
      upsertDirective('error_log', value.trim())
  },
})

const mainLocationContent = computed({
  get() {
    return getMainLocation()?.content ?? ''
  },
  set(value: string) {
    ensureMainLocation().content = value
  },
})

function handleStatusChanged(event: { status: SiteStatus }) {
  data.value.status = event.status
}
</script>

<template>
  <div class="px-6 pb-2">
    <div v-if="showServerSelector" class="server-selector mb-4">
      <span class="mr-3 text-gray-500">{{ $gettext('Server Block') }}</span>
      <ASelect v-model:value="curServerIdx" class="min-w-320px" :options="serverOptions" />
    </div>

    <ATabs
      v-model:active-key="activeModule"
      tab-position="left"
      size="small"
      class="site-basic-tabs"
      :tab-bar-style="{ width: '156px' }"
    >
      <ATabPane key="domain" :tab="$gettext('Domain Settings')">
        <div class="domain-settings-panel">
          <AButton class="mb-4" type="primary" ghost @click="openDomainModal">
            {{ $gettext('Add Domain') }}
          </AButton>

          <ATable
            row-key="key"
            :columns="domainColumns"
            :data-source="domainRows"
            :pagination="false"
            :scroll="{ x: 720 }"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'domain'">
                <div class="domain-cell">
                  <SendOutlined class="domain-cell-icon" />
                  <span>{{ (record as DomainRow).domain }}</span>
                </div>
              </template>
              <template v-else-if="column.key === 'actions'">
                <APopconfirm
                  :title="$gettext('Are you sure you want to delete?')"
                  @confirm="deleteDomain((record as DomainRow).domain)"
                >
                  <AButton type="link" size="small" danger>
                    {{ $gettext('Delete') }}
                  </AButton>
                </APopconfirm>
              </template>
            </template>
          </ATable>

          <AModal
            v-model:open="domainModalVisible"
            :title="$gettext('Add Domain')"
            @ok="handleAddDomain"
          >
            <AForm layout="vertical">
              <AFormItem :label="$gettext('Domain')" required>
                <AInput
                  v-model:value="domainForm.domain"
                  placeholder="example.com"
                  @press-enter="handleAddDomain"
                />
              </AFormItem>
              <AFormItem :label="$gettext('Port')" required>
                <AInputNumber
                  v-model:value="domainForm.port"
                  class="w-full"
                  :min="1"
                  :max="65535"
                />
              </AFormItem>
            </AForm>
          </AModal>
        </div>
      </ATabPane>

      <ATabPane key="directory" :tab="$gettext('Website Directory')">
        <AForm layout="vertical">
          <AFormItem :label="$gettext('Root Directory')">
            <AInput v-model:value="siteRoot" placeholder="/var/www/sites/example.com/index" />
          </AFormItem>
        </AForm>

        <div v-if="normalizedSiteRoot" class="directory-summary">
          <div class="directory-summary-row">
            <span>{{ $gettext('Site Name') }}</span>
            <strong>{{ name }}</strong>
          </div>
          <div class="directory-summary-row">
            <span>{{ $gettext('Site Workspace') }}</span>
            <code>{{ siteWorkspace }}</code>
          </div>
          <div class="directory-summary-row">
            <span>{{ $gettext('Root Directory') }}</span>
            <code>{{ normalizedSiteRoot }}</code>
          </div>
        </div>

        <AAlert
          v-if="normalizedSiteRoot && !hasIndexRootDirectory"
          class="mb-4"
          type="info"
          show-icon
          :message="$gettext('Use an index subdirectory if you want the ssl, log, and index layout.')"
        />

        <template v-if="directoryRows.length">
          <h3 class="directory-section-title">
            {{ $gettext('Site Main Directories') }}
          </h3>
          <ATable
            row-key="key"
            :columns="directoryColumns"
            :data-source="directoryRows"
            :pagination="false"
            :scroll="{ x: 760 }"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'name'">
                <strong>{{ (record as DirectoryRow).name }}</strong>
              </template>
              <template v-else-if="column.key === 'path'">
                <code>{{ (record as DirectoryRow).path }}</code>
              </template>
              <template v-else-if="column.key === 'actions'">
                <AButton type="link" size="small" @click="copyDirectoryPath((record as DirectoryRow).path)">
                  <template #icon>
                    <CopyOutlined />
                  </template>
                  {{ $gettext('Copy') }}
                </AButton>
              </template>
            </template>
          </ATable>
        </template>
      </ATabPane>

      <ATabPane key="default-doc" :tab="$gettext('Default Documents')">
        <AForm layout="vertical">
          <AFormItem :label="$gettext('Default Documents')" required>
            <ATextarea
              v-model:value="indexFilesInput"
              :rows="8"
              :placeholder="defaultDocumentPlaceholder"
              @blur="handleIndexFilesBlur"
            />
          </AFormItem>
        </AForm>

        <AAlert
          type="info"
          show-icon
          :message="$gettext('One default document per line. The generated NGINX index directive will use spaces between files.')"
        />
      </ATabPane>

      <ATabPane key="rate-limit" :tab="$gettext('Rate Limit')">
        <AForm layout="vertical">
          <AFormItem :label="$gettext('Enable Rate Limit')">
            <ASwitch v-model:checked="rateLimitEnabled" />
          </AFormItem>

          <template v-if="rateLimitEnabled">
            <AFormItem :label="$gettext('Limit Scheme')">
              <ASelect v-model:value="rateLimitPlan" :options="rateLimitPlanOptions" />
            </AFormItem>
            <AFormItem
              :label="$gettext('Concurrent Limit')"
              required
              :extra="$gettext('Limit the maximum concurrent connections for the current site')"
            >
              <AInputNumber
                v-model:value="concurrentLimit"
                class="w-full"
                :min="1"
                :precision="0"
                placeholder="300"
              />
            </AFormItem>
            <AFormItem
              :label="$gettext('Single IP Limit')"
              required
              :extra="$gettext('Limit the maximum concurrent connections per IP')"
            >
              <AInputNumber
                v-model:value="singleIPLimit"
                class="w-full"
                :min="1"
                :precision="0"
                placeholder="25"
              />
            </AFormItem>
            <AFormItem
              :label="$gettext('Single Request Rate Limit')"
              required
              :extra="$gettext('Limit transfer speed per request, unit: KB/s')"
            >
              <AInputNumber
                v-model:value="singleRequestRateLimit"
                class="w-full"
                :min="1"
                :precision="0"
                placeholder="512"
              />
            </AFormItem>
            <AAlert
              type="warning"
              show-icon
              :message="$gettext('limit_conn requires matching perserver and perip limit_conn_zone directives in the global NGINX configuration.')"
            />
          </template>
        </AForm>
      </ATabPane>

      <ATabPane key="proxy" :tab="$gettext('Reverse Proxy')">
        <AForm layout="vertical">
          <AFormItem :label="$gettext('Proxy Target')">
            <AInput v-model:value="proxyTarget" placeholder="http://127.0.0.1:3000" />
          </AFormItem>
          <AFormItem :label="$gettext('Location / Content')">
            <ATextarea v-model:value="mainLocationContent" :rows="6" />
          </AFormItem>
        </AForm>
      </ATabPane>

      <ATabPane key="load-balance" :tab="$gettext('Load Balance')">
        <NgxUpstream />
        <AAlert
          class="mt-4"
          type="info"
          show-icon
          :message="$gettext('Use an upstream name as the proxy target, for example http://backend.')"
        />
      </ATabPane>

      <ATabPane key="auth" :tab="$gettext('Password Access')">
        <div class="auth-basic-panel">
          <ATabs active-key="global" size="small" class="auth-basic-tabs">
            <ATabPane key="global" :tab="$gettext('Global')">
              <div class="auth-basic-toolbar">
                <AButton type="primary" ghost @click="openCreateAuthUserDrawer">
                  {{ $gettext('Create') }}
                </AButton>
                <ASwitch
                  :checked="authBasicEnabled"
                  :loading="authBasicSwitching"
                  @change="handleAuthBasicEnabledChange"
                />
              </div>

              <ATable
                row-key="username"
                :columns="authBasicColumns"
                :data-source="authBasicUsers"
                :loading="authBasicLoading"
                :pagination="false"
                :scroll="{ x: 720 }"
              >
                <template #bodyCell="{ column, record }">
                  <template v-if="column.key === 'remark'">
                    {{ (record as BasicAuthUser).remark || '-' }}
                  </template>
                  <template v-else-if="column.key === 'actions'">
                    <ASpace>
                      <AButton type="link" size="small" @click="openEditAuthUserDrawer(record as BasicAuthUser)">
                        {{ $gettext('Edit') }}
                      </AButton>
                      <APopconfirm
                        :title="$gettext('Are you sure you want to delete?')"
                        @confirm="deleteAuthUser((record as BasicAuthUser).username)"
                      >
                        <AButton type="link" size="small" danger>
                          {{ $gettext('Delete') }}
                        </AButton>
                      </APopconfirm>
                    </ASpace>
                  </template>
                </template>
              </ATable>
            </ATabPane>
          </ATabs>
        </div>

        <ADrawer
          v-model:open="authUserDrawerVisible"
          :title="authUserDrawerTitle"
          :width="authUserDrawerWidth"
          destroy-on-close
        >
          <AForm layout="vertical" class="auth-user-form">
            <AFormItem :label="$gettext('Username')" required>
              <AInput
                v-model:value="authUserForm.username"
                :disabled="authUserMode === 'edit'"
                @press-enter="submitAuthUser"
              />
            </AFormItem>

            <AFormItem
              :label="$gettext('Password')"
              :required="authUserMode === 'create'"
              :extra="authUserMode === 'edit' ? $gettext('Leave blank to keep the current password') : undefined"
            >
              <AInputPassword v-model:value="authUserForm.password" @press-enter="submitAuthUser">
                <template #addonAfter>
                  <AButton type="link" size="small" @click="fillRandomAuthPassword">
                    {{ $gettext('Random Password') }}
                  </AButton>
                </template>
              </AInputPassword>
            </AFormItem>

            <AFormItem :label="$gettext('Remark')">
              <AInput v-model:value="authUserForm.remark" @press-enter="submitAuthUser" />
            </AFormItem>
          </AForm>

          <template #footer>
            <ASpace class="auth-user-drawer-footer">
              <AButton @click="authUserDrawerVisible = false">
                {{ $gettext('Cancel') }}
              </AButton>
              <AButton type="primary" :loading="authUserSubmitting" @click="submitAuthUser">
                {{ $gettext('Confirm') }}
              </AButton>
            </ASpace>
          </template>
        </ADrawer>
      </ATabPane>

      <ATabPane key="cors" :tab="$gettext('Cross-domain Access')">
        <AForm
          class="site-cors-form"
          layout="horizontal"
          :label-col="{ style: { width: '140px' } }"
          :wrapper-col="{ flex: 1 }"
        >
          <AFormItem :label="$gettext('Enable CORS')">
            <ASwitch v-model:checked="corsEnabled" />
          </AFormItem>

          <template v-if="corsEnabled">
            <AFormItem :label="$gettext('Allowed Domains')" required>
              <AInput v-model:value="corsAllowOrigin" placeholder="*" />
            </AFormItem>
            <AFormItem :label="$gettext('Allowed Request Methods')">
              <AInput v-model:value="corsAllowMethods" />
            </AFormItem>
            <AFormItem :label="$gettext('Allowed Request Headers')">
              <ATextarea v-model:value="corsAllowHeaders" :rows="3" />
            </AFormItem>
            <AFormItem :label="$gettext('Allow Credentials')">
              <ASwitch v-model:checked="corsAllowCredentials" />
            </AFormItem>
            <AFormItem
              :label="$gettext('Preflight Quick Response')"
              :extra="$gettext('After enabling, when the browser sends a cross-origin preflight request (OPTIONS request), NGINX automatically returns status 204 and adds the required CORS response headers.')"
            >
              <ASwitch v-model:checked="corsPreflightQuickResponse" />
            </AFormItem>
          </template>
        </AForm>
      </ATabPane>

      <ATabPane key="https" tab="HTTPS">
        <HttpsSettings />
      </ATabPane>

      <ATabPane key="real-ip" :tab="$gettext('Real IP')">
        <div class="real-ip-description">
          <p>{{ $gettext('Configure trusted IP sources so NGINX can read visitor IP information from HTTP headers and record the real client IP, including in access logs.') }}</p>
          <p>{{ $gettext('If the frontend is FRP or another tool, enter the frontend proxy IP address, for example 127.0.0.1.') }}</p>
          <p>{{ $gettext('If the frontend is a CDN, enter the CDN IP ranges.') }}</p>
          <p>{{ $gettext('If you are unsure, you can enter 0.0.0.0/0 (IPv4) and ::/0 (IPv6). Warning: trusting all sources is unsafe.') }}</p>
        </div>

        <AForm
          layout="horizontal"
          :label-col="{ style: { width: '96px' } }"
          :wrapper-col="{ span: 20 }"
        >
          <AFormItem :label="$gettext('Enable')">
            <ASwitch v-model:checked="realIpEnabled" />
          </AFormItem>

          <template v-if="realIpEnabled">
            <AFormItem
              :label="$gettext('IP Source')"
              required
              :extra="$gettext('Enter one item per line')"
            >
              <ATextarea
                v-model:value="realIpFrom"
                :rows="8"
                placeholder="127.0.0.1"
              />
            </AFormItem>
            <AFormItem :label="$gettext('IP Header')" required>
              <ASelect v-model:value="realIpHeader" :options="realIpHeaderOptions" />
            </AFormItem>
          </template>
        </AForm>
      </ATabPane>

      <ATabPane key="rewrite" :tab="$gettext('Pseudo-static')">
        <AForm layout="vertical">
          <AFormItem :label="$gettext('Rewrite Rules')">
            <ATextarea
              v-model:value="rewriteRules"
              :rows="8"
              placeholder="^/old/(.*)$ /new/$1 permanent"
            />
          </AFormItem>
        </AForm>
      </ATabPane>

      <ATabPane key="anti-leech" :tab="$gettext('Anti-leech')">
        <AForm layout="vertical">
          <AFormItem>
            <ACheckbox v-model:checked="antiLeechEnabled">
              {{ $gettext('Enable Anti-leech') }}
            </ACheckbox>
          </AFormItem>
          <AFormItem :label="$gettext('Valid Referers')">
            <ATextarea v-model:value="antiLeechReferers" :rows="3" />
          </AFormItem>
        </AForm>
      </ATabPane>

      <ATabPane key="redirect" :tab="$gettext('Redirect')">
        <AForm layout="vertical">
          <AFormItem :label="$gettext('Return Rule')">
            <AInput v-model:value="redirectReturn" placeholder="301 https://example.com$request_uri" />
          </AFormItem>
        </AForm>
      </ATabPane>

      <ATabPane key="other" :tab="$gettext('Other')">
        <ACollapse ghost>
          <ACollapsePanel key="site" :header="$gettext('Basic Information')">
            <AForm layout="vertical">
              <AFormItem :label="$gettext('Status')">
                <SiteStatusSelect
                  v-model="data.status"
                  :site-name="name"
                  @status-changed="handleStatusChanged"
                />
              </AFormItem>
              <AFormItem :label="$gettext('Name')">
                <ConfigName v-if="name" :name />
              </AFormItem>
              <AFormItem :label="$gettext('Updated at')">
                {{ formatDateTime(data.modified_at) }}
              </AFormItem>
              <AFormItem :label="$gettext('Namespace')">
                <StdSelector
                  v-model:value="data.namespace_id"
                  :get-list-api="namespace.getList"
                  :columns="namespaceColumns"
                  display-key="name"
                  selection-type="radio"
                />
              </AFormItem>
            </AForm>
          </ACollapsePanel>

          <ACollapsePanel key="dns" header="DNS">
            <DNS />
          </ACollapsePanel>

          <ACollapsePanel v-if="!advanceMode" key="config-template" :header="$gettext('Config Template')">
            <ConfigTemplatePanel />
          </ACollapsePanel>

          <ACollapsePanel key="chat" :header="$gettext('Chat')">
            <Chat chat-height="calc(100vh - 320px)" />
          </ACollapsePanel>

          <ACollapsePanel key="port-scanner" :header="$gettext('Port Scanner')">
            <PortScannerCompact />
          </ACollapsePanel>

          <ACollapsePanel key="logs" :header="$gettext('Logs')">
            <AForm layout="vertical">
              <AFormItem>
                <ACheckbox v-model:checked="hasAccessLog">
                  {{ $gettext('Access Log') }}
                </ACheckbox>
              </AFormItem>
              <AFormItem v-if="hasAccessLog" :label="$gettext('Access Log Path')">
                <AInput v-model:value="accessLogPath" />
              </AFormItem>
              <AFormItem>
                <ACheckbox v-model:checked="hasErrorLog">
                  {{ $gettext('Error Log') }}
                </ACheckbox>
              </AFormItem>
              <AFormItem v-if="hasErrorLog" :label="$gettext('Error Log Path')">
                <AInput v-model:value="errorLogPath" />
              </AFormItem>
            </AForm>
          </ACollapsePanel>

          <ACollapsePanel v-if="!settings.is_remote" key="sync" :header="$gettext('Synchronization')">
            <div>
              <div class="flex items-center justify-between mb-4">
                <div>
                  {{ $gettext('Synchronization') }}
                </div>
                <APopover placement="bottomRight" :title="$gettext('Sync strategy')">
                  <template #content>
                    <div class="max-w-200px mb-2">
                      {{ $gettext('When you enable/disable, delete, or save this site, '
                        + 'the nodes set in the namespace and the nodes selected below will be synchronized.') }}
                    </div>
                    <div class="max-w-200px">
                      {{ $gettext('Note, if the configuration file include other configurations or certificates, '
                        + 'please synchronize them to the remote nodes in advance.') }}
                    </div>
                  </template>
                  <div class="text-trueGray-600">
                    <InfoCircleOutlined class="mr-1" />
                    {{ $gettext('Sync strategy') }}
                  </div>
                </APopover>
              </div>
              <NodeSelector
                v-model:target="data.sync_node_ids"
                class="mb-4"
                hidden-local
              />

              <SyncNodesPreview
                :namespace-id="data.namespace_id"
                :sync-node-ids="data.sync_node_ids"
              />
            </div>
          </ACollapsePanel>
        </ACollapse>
      </ATabPane>
    </ATabs>
  </div>
</template>

<style scoped lang="less">
:deep(.ant-collapse-ghost > .ant-collapse-item > .ant-collapse-content > .ant-collapse-content-box) {
  padding: 0;
}

:deep(.ant-collapse > .ant-collapse-item > .ant-collapse-header) {
  padding: 0 0 10px 0;
}

.site-basic-tabs {
  min-height: 520px;

  :deep(.ant-tabs-nav-list) {
    width: 100%;
  }

  :deep(.ant-tabs-tab) {
    justify-content: flex-start;
    margin: 0 !important;
    padding: 10px 12px;
    white-space: nowrap;
  }

  :deep(.ant-tabs-tab + .ant-tabs-tab) {
    margin-top: 4px !important;
  }

  :deep(.ant-tabs-tab-btn) {
    overflow: hidden;
    text-overflow: ellipsis;
  }

  :deep(.ant-tabs-content-holder) {
    padding-left: 24px;
    min-width: 0;
  }

  :deep(.ant-form) {
    max-width: 760px;
  }
}

.site-basic-tabs :deep(.site-cors-form.ant-form) {
  max-width: none;
}

@media (max-width: 640px) {
  .site-basic-tabs :deep(.site-cors-form .ant-form-item) {
    display: block;
  }

  .site-basic-tabs :deep(.site-cors-form .ant-form-item-label) {
    width: auto !important;
    text-align: left;
  }
}

.domain-cell {
  display: flex;
  align-items: center;
  gap: 12px;
}

.domain-cell-icon {
  color: #6b7280;
  font-size: 14px;
}

.auth-basic-panel {
  border: 1px solid #e5e7eb;
}

.auth-basic-tabs {
  :deep(.ant-tabs-nav) {
    margin: 0;
    padding: 0 14px;
    background: #f5f7fb;
  }

  :deep(.ant-tabs-content-holder) {
    padding: 16px 14px 12px;
  }
}

.auth-basic-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.auth-user-form {
  max-width: none !important;
}

.auth-user-drawer-footer {
  display: flex;
  justify-content: flex-end;
}

.directory-summary {
  display: grid;
  gap: 14px;
  margin-bottom: 20px;
  max-width: 760px;
}

.directory-summary-row {
  display: grid;
  grid-template-columns: 120px minmax(0, 1fr);
  align-items: center;
  gap: 16px;

  span {
    color: #6b7280;
  }

  code {
    overflow-wrap: anywhere;
  }
}

.directory-section-title {
  margin: 0 0 16px;
  font-size: 16px;
  font-weight: 600;
}
</style>
