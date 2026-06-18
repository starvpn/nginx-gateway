<script setup lang="ts">
import type { NgxDirective, NgxLocation, NgxServer } from '@/api/ngx'
import type { SiteStatus } from '@/api/site'
import { InfoCircleOutlined } from '@ant-design/icons-vue'
import { StdSelector } from '@uozi-admin/curd'
import namespace from '@/api/namespace'
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

const editorStore = useSiteEditorStore()
const { name, data, ngxConfig, curServerIdx, curServer, curServerDirectives, advanceMode } = storeToRefs(editorStore)

const activeModule = ref('site')
const modulesWithoutServerSelector = ['https', 'dns', 'config-template', 'chat', 'port-scanner', 'load-balance']

const serverOptions = computed(() => {
  return (ngxConfig.value.servers ?? []).map((server, index) => {
    const serverName = server.directives?.find(item => item.directive === 'server_name')?.params
    const listens = server.directives?.filter(item => item.directive === 'listen').map(item => item.params).join(', ')
    const label = [
      $gettext('Server %{n}', { n: index + 1 }),
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
    return params.includes('ssl') === isSSL && params.trim().startsWith('[::]') === isIPv6
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
  const match = params.match(/(?:\[::\]:)?(\d+)/)
  return match?.[1] ?? ''
}

function syncIPv6Listen(enabled: boolean) {
  const directives = ensureDirectives()
  const httpsParams = getListenDirective(true)?.params ?? ''
  const httpPort = getListenPort(getListenDirective(false)?.params) || '80'
  const httpsPort = getListenPort(httpsParams) || '443'

  for (let i = directives.length - 1; i >= 0; i--) {
    if (directives[i].directive === 'listen' && directives[i].params.trim().startsWith('[::]'))
      directives.splice(i, 1)
  }

  if (!enabled)
    return

  directives.unshift({ directive: 'listen', params: `[::]:${httpPort}` })

  if (httpsParams)
    directives.unshift({ directive: 'listen', params: `[::]:${httpsPort} ssl` })
}

function setDefaultServer(enabled: boolean) {
  findDirectives('listen').forEach(item => {
    const parts = item.params.split(/\s+/).filter(Boolean)
    const next = parts.filter(part => part !== 'default_server')
    if (enabled)
      next.push('default_server')
    item.params = next.join(' ')
  })
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

const clientMaxBodySize = computed({
  get() {
    return findDirective('client_max_body_size')?.params ?? ''
  },
  set(value: string) {
    setOptionalDirective('client_max_body_size', value)
  },
})

const limitRate = computed({
  get() {
    return findDirective('limit_rate')?.params ?? ''
  },
  set(value: string) {
    setOptionalDirective('limit_rate', value)
  },
})

const limitConn = computed({
  get() {
    return findDirective('limit_conn')?.params ?? ''
  },
  set(value: string) {
    setOptionalDirective('limit_conn', value)
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

const corsAllowOrigin = ref('*')
const corsAllowMethods = ref('GET, POST, PUT, PATCH, DELETE, OPTIONS')
const corsAllowHeaders = ref('Authorization, Content-Type, Accept, Origin, User-Agent')
const corsAllowCredentials = ref(false)

const corsEnabled = computed({
  get() {
    return hasManagedLocationBlock(corsBlockStart, corsBlockEnd)
  },
  set(value: boolean) {
    if (!value) {
      removeManagedLocationBlock(corsBlockStart, corsBlockEnd)
      return
    }

    setManagedLocationBlock(corsBlockStart, corsBlockEnd, [
      `add_header Access-Control-Allow-Origin "${corsAllowOrigin.value}" always;`,
      `add_header Access-Control-Allow-Methods "${corsAllowMethods.value}" always;`,
      `add_header Access-Control-Allow-Headers "${corsAllowHeaders.value}" always;`,
      `add_header Access-Control-Allow-Credentials "${corsAllowCredentials.value ? 'true' : 'false'}" always;`,
      'if ($request_method = OPTIONS) { return 204; }',
    ].join('\n'))
  },
})

watch([corsAllowOrigin, corsAllowMethods, corsAllowHeaders, corsAllowCredentials], () => {
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

const realIpRecursive = computed({
  get() {
    return findDirective('real_ip_recursive')?.params === 'on'
  },
  set(value: boolean) {
    setOptionalDirective('real_ip_recursive', value ? 'on' : '')
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

const serverNames = computed({
  get() {
    return findDirective('server_name')?.params ?? ''
  },
  set(value: string) {
    upsertDirective('server_name', value.trim())
  },
})

const httpListen = computed({
  get() {
    return getListenDirective(false)?.params ?? '80'
  },
  set(value: string) {
    upsertListen(false, value.trim() || '80')
  },
})

const isIPv6Enabled = computed({
  get() {
    return findDirectives('listen').some(item => item.params.trim().startsWith('[::]'))
  },
  set(value: boolean) {
    syncIPv6Listen(value)
  },
})

const isDefaultServer = computed({
  get() {
    return findDirectives('listen').some(item => item.params.includes('default_server'))
  },
  set(value: boolean) {
    setDefaultServer(value)
  },
})

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

const indexFiles = computed({
  get() {
    return findDirective('index')?.params ?? ''
  },
  set(value: string) {
    if (value.trim())
      upsertDirective('index', value.trim())
    else
      removeDirective('index')
  },
})

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
      <ATabPane key="site" :tab="$gettext('Site')">
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
      </ATabPane>

      <ATabPane key="domain" :tab="$gettext('Domain Settings')">
        <AForm layout="vertical">
          <AFormItem :label="$gettext('Server Names')">
            <ATextarea
              v-model:value="serverNames"
              :rows="2"
              placeholder="example.com www.example.com 192.168.1.10"
            />
          </AFormItem>
          <AFormItem :label="$gettext('HTTP Listen')">
            <AInput v-model:value="httpListen" placeholder="80" />
          </AFormItem>
          <AFlex gap="large" wrap="wrap">
            <ACheckbox v-model:checked="isIPv6Enabled">
              {{ $gettext('IPv6') }}
            </ACheckbox>
            <ACheckbox v-model:checked="isDefaultServer">
              {{ $gettext('Default Server') }}
            </ACheckbox>
          </AFlex>
          <AAlert
            class="mt-4"
            type="info"
            show-icon
            :message="$gettext('For IP access with a custom port, put the IP in server_name and set the port in listen, for example listen 8080.')"
          />
        </AForm>
      </ATabPane>

      <ATabPane key="https" tab="HTTPS">
        <HttpsSettings />
      </ATabPane>

      <ATabPane key="dns" tab="DNS">
        <DNS />
      </ATabPane>

      <ATabPane v-if="!advanceMode" key="config-template" :tab="$gettext('Config Template')">
        <ConfigTemplatePanel />
      </ATabPane>

      <ATabPane key="chat" :tab="$gettext('Chat')">
        <Chat chat-height="calc(100vh - 320px)" />
      </ATabPane>

      <ATabPane key="port-scanner" :tab="$gettext('Port Scanner')">
        <PortScannerCompact />
      </ATabPane>

      <ATabPane key="directory" :tab="$gettext('Website Directory')">
        <AForm layout="vertical">
          <AFormItem :label="$gettext('Site Directory')">
            <AInput v-model:value="siteRoot" placeholder="/var/www/example.com" />
          </AFormItem>
        </AForm>
      </ATabPane>

      <ATabPane key="default-doc" :tab="$gettext('Default Documents')">
        <AForm layout="vertical">
          <AFormItem :label="$gettext('Index Files')">
            <ATextarea
              v-model:value="indexFiles"
              :rows="5"
              placeholder="index.html index.htm"
            />
          </AFormItem>
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

      <ATabPane key="rate-limit" :tab="$gettext('Rate Limit')">
        <AForm layout="vertical">
          <AFormItem :label="$gettext('Client Max Body Size')">
            <AInput v-model:value="clientMaxBodySize" placeholder="50m" />
          </AFormItem>
          <AFormItem :label="$gettext('Limit Rate')">
            <AInput v-model:value="limitRate" placeholder="1m" />
          </AFormItem>
          <AFormItem :label="$gettext('Limit Conn')">
            <AInput v-model:value="limitConn" placeholder="addr 10" />
          </AFormItem>
          <AAlert
            type="warning"
            show-icon
            :message="$gettext('limit_conn requires a matching limit_conn_zone in the global nginx configuration.')"
          />
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

      <ATabPane key="auth" :tab="$gettext('Basic Auth')">
        <AForm layout="vertical">
          <AFormItem :label="$gettext('Auth Realm')">
            <AInput v-model:value="authBasic" placeholder="Restricted" />
          </AFormItem>
          <AFormItem :label="$gettext('Password File')">
            <AInput v-model:value="authBasicUserFile" placeholder="/etc/nginx/.htpasswd" />
          </AFormItem>
        </AForm>
      </ATabPane>

      <ATabPane key="cors" :tab="$gettext('CORS')">
        <AForm layout="vertical">
          <AFormItem>
            <ACheckbox v-model:checked="corsEnabled">
              {{ $gettext('Enable CORS') }}
            </ACheckbox>
          </AFormItem>
          <AFormItem :label="$gettext('Allow Origin')">
            <AInput v-model:value="corsAllowOrigin" placeholder="*" />
          </AFormItem>
          <AFormItem :label="$gettext('Allow Methods')">
            <AInput v-model:value="corsAllowMethods" />
          </AFormItem>
          <AFormItem :label="$gettext('Allow Headers')">
            <ATextarea v-model:value="corsAllowHeaders" :rows="3" />
          </AFormItem>
          <AFormItem>
            <ACheckbox v-model:checked="corsAllowCredentials">
              {{ $gettext('Allow Credentials') }}
            </ACheckbox>
          </AFormItem>
        </AForm>
      </ATabPane>

      <ATabPane key="real-ip" :tab="$gettext('Real IP')">
        <AForm layout="vertical">
          <AFormItem :label="$gettext('Trusted Proxies')">
            <ATextarea
              v-model:value="realIpFrom"
              :rows="4"
              placeholder="127.0.0.1\n10.0.0.0/8"
            />
          </AFormItem>
          <AFormItem :label="$gettext('Real IP Header')">
            <AInput v-model:value="realIpHeader" placeholder="X-Forwarded-For" />
          </AFormItem>
          <AFormItem>
            <ACheckbox v-model:checked="realIpRecursive">
              {{ $gettext('Real IP Recursive') }}
            </ACheckbox>
          </AFormItem>
        </AForm>
      </ATabPane>

      <ATabPane key="rewrite" :tab="$gettext('Rewrite')">
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

      <ATabPane key="logs" :tab="$gettext('Logs')">
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
      </ATabPane>

      <ATabPane v-if="!settings.is_remote" key="sync" :tab="$gettext('Synchronization')">
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
</style>
