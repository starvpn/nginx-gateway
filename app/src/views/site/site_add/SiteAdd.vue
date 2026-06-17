<script setup lang="ts">
import type { DNSDomain, DNSRecord } from '@/api/dns'
import type { NgxDirective, NgxServer } from '@/api/ngx'
import type { CreateSiteRequest, SiteType } from '@/api/site'
import ngx from '@/api/ngx'
import site from '@/api/site'
import NgxConfigEditor, { DirectiveEditor, LocationEditor, useNgxConfigStore } from '@/components/NgxConfigEditor'
import DNSRecordIntegration from './components/DNSRecordIntegration.vue'

interface SiteForm {
  type: SiteType
  name: string
  primary_domain: string
  domains: string
  remark: string
  site_dir: string
  index: string
  proxy_target: string
  enable_ipv6: boolean
  access_log: boolean
  error_log: boolean
}

const router = useRouter()
const currentStep = ref(0)
const isSaving = ref(false)
const createdName = ref('')
const selectedDNSRecord = ref<{ record: DNSRecord, domain: DNSDomain } | null>(null)
const { message } = useGlobalApp()

const form = reactive<SiteForm>({
  type: 'reverse_proxy',
  name: '',
  primary_domain: '',
  domains: '',
  remark: '',
  site_dir: '',
  index: 'index.html index.htm',
  proxy_target: 'http://127.0.0.1:3000',
  enable_ipv6: true,
  access_log: true,
  error_log: true,
})

const ngxConfigStore = useNgxConfigStore()
const { ngxConfig, curServerDirectives, curServerLocations } = storeToRefs(ngxConfigStore)

onMounted(() => {
  initCustomConfig()
})

function initCustomConfig() {
  ngxConfigStore.reset()
  site.get_default_template().then(r => {
    ngxConfigStore.setNgxConfig(r.tokenized)
  })
}

watch(() => form.type, type => {
  currentStep.value = 0
  selectedDNSRecord.value = null
  if (type === 'custom')
    initCustomConfig()
})

watch(() => form.primary_domain, value => {
  if (!form.name)
    form.name = value
})

const isCustomMode = computed(() => form.type === 'custom')

const serverNameValue = computed(() => {
  const servers = ngxConfig.value.servers

  for (const server of Object.values(servers) as NgxServer[]) {
    if (!server.directives)
      continue

    for (const directive of Object.values(server.directives) as NgxDirective[]) {
      if (directive.directive === 'server_name' && directive.params.trim()) {
        const names = directive.params.trim().split(/\s+/)
        return names[0] || ''
      }
    }
  }

  return ''
})

const hasServerName = computed(() => {
  return Boolean(serverNameValue.value)
})

const primaryDomain = computed(() => {
  if (isCustomMode.value)
    return serverNameValue.value
  return form.primary_domain.trim()
})

const canContinueFromBasic = computed(() => {
  if (isCustomMode.value)
    return Boolean(ngxConfig.value.name && hasServerName.value)

  if (!form.primary_domain.trim())
    return false

  if (form.type === 'static')
    return true

  return Boolean(form.proxy_target.trim())
})

const domainList = computed(() => {
  return form.domains
    .split(/[\s,]+/)
    .map(v => v.trim())
    .filter(Boolean)
})

function getFullDNSName(record: DNSRecord, domain: DNSDomain): string {
  if (record.name === '@' || record.name === domain.domain)
    return domain.domain
  return `${record.name}.${domain.domain}`
}

function updateServerNameWithDNS(dnsName: string) {
  if (!isCustomMode.value) {
    form.primary_domain = dnsName
    return
  }

  for (const server of Object.values(ngxConfig.value.servers) as NgxServer[]) {
    if (!server.directives)
      continue

    for (const directive of Object.values(server.directives) as NgxDirective[]) {
      if (directive.directive === 'server_name') {
        directive.params = dnsName
        return
      }
    }
  }
}

function onDNSRecordSelected(record: DNSRecord, domain: DNSDomain) {
  selectedDNSRecord.value = { record, domain }
  updateServerNameWithDNS(getFullDNSName(record, domain))
}

function onDNSRecordCreated(record: DNSRecord, domain: DNSDomain) {
  selectedDNSRecord.value = { record, domain }
  updateServerNameWithDNS(getFullDNSName(record, domain))
  message.success($gettext('DNS record created and linked successfully'))
}

function onDNSRecordCleared() {
  selectedDNSRecord.value = null
}

function buildPayload(customContent = ''): CreateSiteRequest {
  const payload: CreateSiteRequest = {
    name: isCustomMode.value ? ngxConfig.value.name : form.name,
    type: form.type,
    primary_domain: primaryDomain.value,
    domains: isCustomMode.value ? [] : domainList.value,
    remark: form.remark,
    site_dir: form.site_dir,
    index: form.index,
    proxy_target: form.proxy_target,
    enable_ssl: false,
    enable_ipv6: form.enable_ipv6,
    access_log: form.access_log,
    error_log: form.error_log,
    custom_content: customContent,
    namespace_id: 0,
    sync_node_ids: [],
    overwrite: false,
    post_action: 'reload_nginx',
  }

  if (selectedDNSRecord.value) {
    payload.dns_domain_id = selectedDNSRecord.value.domain.id
    payload.dns_record_id = selectedDNSRecord.value.record.id
    payload.dns_record_name = selectedDNSRecord.value.record.name
    payload.dns_record_type = selectedDNSRecord.value.record.type
  }

  return payload
}

async function save() {
  isSaving.value = true
  try {
    let customContent = ''
    if (isCustomMode.value) {
      const r = await ngx.build_config(ngxConfig.value)
      customContent = r.content
    }

    const response = await site.create(buildPayload(customContent))
    createdName.value = response.name
    message.success($gettext('Saved successfully'))

    await site.enable(response.name)
    message.success($gettext('Enabled successfully'))

    currentStep.value = 3
    window.scroll({ top: 0, left: 0, behavior: 'smooth' })
  }
  finally {
    isSaving.value = false
  }
}

async function next() {
  if (currentStep.value === 2) {
    await save()
    return
  }
  currentStep.value++
}

function gotoModify() {
  router.push(`/sites/${encodeURIComponent(createdName.value)}`)
}

function createAnother() {
  router.go(0)
}
</script>

<template>
  <ACard :title="$gettext('Add Site')">
    <div class="site-add-container">
      <ASteps
        :current="currentStep"
        size="small"
      >
        <AStep :title="$gettext('Base information')" />
        <AStep :title="$gettext('DNS Record')" />
        <AStep :title="$gettext('Review')" />
        <AStep :title="$gettext('Finished')" />
      </ASteps>

      <div v-if="currentStep === 0" class="mt-6">
        <AForm layout="vertical">
          <AFormItem :label="$gettext('Site Type')">
            <ARadioGroup v-model:value="form.type" button-style="solid">
              <ARadioButton value="reverse_proxy">
                {{ $gettext('Reverse Proxy') }}
              </ARadioButton>
              <ARadioButton value="static">
                {{ $gettext('Static Site') }}
              </ARadioButton>
              <ARadioButton value="custom">
                {{ $gettext('Custom Config') }}
              </ARadioButton>
            </ARadioGroup>
          </AFormItem>

          <template v-if="!isCustomMode">
            <AFormItem :label="$gettext('Primary Domain')" required>
              <AInput
                v-model:value="form.primary_domain"
                placeholder="example.com"
              />
            </AFormItem>

            <AFormItem :label="$gettext('Configuration Name')">
              <AInput
                v-model:value="form.name"
                :placeholder="form.primary_domain || 'example.com'"
              />
            </AFormItem>

            <AFormItem :label="$gettext('Other Domains')">
              <ATextarea
                v-model:value="form.domains"
                :rows="2"
                placeholder="www.example.com, api.example.com"
              />
            </AFormItem>

            <AFormItem
              v-if="form.type === 'static'"
              :label="$gettext('Site Directory')"
            >
              <AInput
                v-model:value="form.site_dir"
                :placeholder="`/var/www/${form.primary_domain || 'example.com'}`"
              />
            </AFormItem>

            <AFormItem
              v-if="form.type === 'static'"
              :label="$gettext('Index Files')"
            >
              <AInput v-model:value="form.index" />
            </AFormItem>

            <AFormItem
              v-if="form.type === 'reverse_proxy'"
              :label="$gettext('Proxy Target')"
              required
            >
              <AInput
                v-model:value="form.proxy_target"
                placeholder="http://127.0.0.1:3000"
              />
            </AFormItem>

            <AFormItem :label="$gettext('Remark')">
              <ATextarea
                v-model:value="form.remark"
                :rows="2"
              />
            </AFormItem>

            <AFlex gap="large" wrap="wrap">
              <ACheckbox v-model:checked="form.enable_ipv6">
                {{ $gettext('IPv6') }}
              </ACheckbox>
              <ACheckbox v-model:checked="form.access_log">
                {{ $gettext('Access Log') }}
              </ACheckbox>
              <ACheckbox v-model:checked="form.error_log">
                {{ $gettext('Error Log') }}
              </ACheckbox>
            </AFlex>
          </template>

          <template v-else>
            <AFormItem :label="$gettext('Configuration Name')" required>
              <AInput v-model:value="ngxConfig.name" />
            </AFormItem>

            <AAlert
              v-if="!hasServerName"
              type="warning"
              class="mb-4"
              show-icon
              :message="$gettext('The parameter of server_name is required')"
            />

            <DirectiveEditor
              v-model:directives="curServerDirectives"
              class="mb-4"
            />
            <LocationEditor
              v-model:locations="curServerLocations"
              :current-server-index="0"
            />
          </template>
        </AForm>
      </div>

      <div v-else-if="currentStep === 1" class="mt-6">
        <DNSRecordIntegration
          v-if="primaryDomain"
          :server-name="primaryDomain"
          @record-created="onDNSRecordCreated"
          @record-selected="onDNSRecordSelected"
          @cleared="onDNSRecordCleared"
        />
        <AEmpty
          v-else
          :description="$gettext('Please configure server_name directive in the configuration before linking DNS records.')"
        />
      </div>

      <div v-else-if="currentStep === 2" class="mt-6">
        <ADescriptions bordered :column="1" size="small">
          <ADescriptionsItem :label="$gettext('Site Type')">
            {{ form.type === 'reverse_proxy' ? $gettext('Reverse Proxy') : form.type === 'static' ? $gettext('Static Site') : $gettext('Custom Config') }}
          </ADescriptionsItem>
          <ADescriptionsItem :label="$gettext('Configuration Name')">
            {{ isCustomMode ? ngxConfig.name : form.name || form.primary_domain }}
          </ADescriptionsItem>
          <ADescriptionsItem :label="$gettext('Primary Domain')">
            {{ primaryDomain }}
          </ADescriptionsItem>
          <ADescriptionsItem v-if="form.type === 'static'" :label="$gettext('Site Directory')">
            {{ form.site_dir || `/var/www/${form.primary_domain}` }}
          </ADescriptionsItem>
          <ADescriptionsItem v-if="form.type === 'reverse_proxy'" :label="$gettext('Proxy Target')">
            {{ form.proxy_target }}
          </ADescriptionsItem>
          <ADescriptionsItem :label="$gettext('DNS Record')">
            {{ selectedDNSRecord ? selectedDNSRecord.record.name : $gettext('Skip') }}
          </ADescriptionsItem>
        </ADescriptions>

        <AAlert
          class="mt-4"
          type="info"
          show-icon
          :message="$gettext('You can configure HTTPS certificates after the site is created.')"
        />

        <NgxConfigEditor v-if="isCustomMode" class="mt-4" />
      </div>

      <ASpace v-if="currentStep < 3" class="mt-6">
        <AButton
          v-if="currentStep > 0"
          @click="currentStep--"
        >
          {{ $gettext('Back') }}
        </AButton>
        <AButton
          type="primary"
          :loading="isSaving"
          :disabled="currentStep === 0 && !canContinueFromBasic"
          @click="next"
        >
          {{ currentStep === 2 ? $gettext('Create') : $gettext('Next') }}
        </AButton>
      </ASpace>

      <AResult
        v-else
        status="success"
        :title="$gettext('Site Config Created Successfully')"
        :sub-title="selectedDNSRecord ? $gettext('DNS record has been linked: %{name}').replace('%{name}', selectedDNSRecord.record.name) : undefined"
      >
        <template #extra>
          <AButton
            type="primary"
            @click="gotoModify"
          >
            {{ $gettext('Modify Config') }}
          </AButton>
          <AButton @click="createAnother">
            {{ $gettext('Create Another') }}
          </AButton>
        </template>
      </AResult>
    </div>
  </ACard>
</template>

<style scoped>
.site-add-container {
  max-width: 920px;
}
</style>
