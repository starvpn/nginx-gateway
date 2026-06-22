<script setup lang="ts">
import type { CheckedType } from '@/types'
import { ArrowLeftOutlined, HistoryOutlined } from '@ant-design/icons-vue'
import CodeEditor from '@/components/CodeEditor/CodeEditor.vue'
import ConfigHistory from '@/components/ConfigHistory'
import FooterToolBar from '@/components/FooterToolbar'
import InspectConfig from '@/components/InspectConfig'
import { ConfigStatus } from '@/constants'
import BasicSettings from '@/views/site/site_edit/components/RightPanel/Basic.vue'
import { useSiteEditorStore } from './components/SiteEditor/store'

const { message } = App.useApp()
const route = useRoute()
const router = useRouter()

const name = computed(() => decodeURIComponent(route.params?.name?.toString() ?? ''))

const editorStore = useSiteEditorStore()
const {
  data,
  parseErrorStatus,
  parseErrorMessage,
  filepath,
  configText,
  loading,
  saving,
  advanceMode,
  dnsLinked,
  linkedDNSName,
} = storeToRefs(editorStore)

provide('dnsLinked', dnsLinked)
provide('linkedDNSName', linkedDNSName)

const activeKey = ref('basic')
const showHistory = ref(false)
const configFileTouched = ref(false)

const inspectConfigRef = useTemplateRef<InstanceType<typeof InspectConfig>>('inspectConfig')

onMounted(() => {
  editorStore.init(name.value)
})

watch(activeKey, async key => {
  if (key === 'config' && !advanceMode.value && !parseErrorStatus.value && !configFileTouched.value) {
    try {
      await editorStore.buildConfig()
    }
    catch {
      // Keep the existing editor content when config generation fails.
    }
  }
})

async function save() {
  try {
    await editorStore.save({
      forceBuildConfig: activeKey.value === 'basic',
      forceConfigText: activeKey.value === 'config' && (advanceMode.value || configFileTouched.value),
    })
    configFileTouched.value = false
    message.success($gettext('Saved successfully'))
    inspectConfigRef.value?.test()
  }
  catch {
    // Error details are surfaced by the store and request layer.
  }
}

async function handleModeChange(checked: CheckedType) {
  await editorStore.handleModeChange(checked)
  if (checked)
    activeKey.value = 'config'
  else
    configFileTouched.value = false
}

function handleConfigContentUpdate(value: string) {
  const changedByEditor = value !== configText.value
  configText.value = value
  if (activeKey.value === 'config' && changedByEditor)
    configFileTouched.value = true
}
</script>

<template>
  <div class="site-edit-page">
    <ACard class="site-edit-card" :bordered="false" :loading>
      <div class="site-edit-header">
        <AButton type="text" class="back-button" @click="router.push('/sites/list')">
          <template #icon>
            <ArrowLeftOutlined />
          </template>
          {{ $gettext('Back') }}
        </AButton>

        <ADivider type="vertical" />

        <ATabs v-model:active-key="activeKey" class="main-tabs" size="large">
          <ATabPane key="basic" :tab="$gettext('Basic')" />
          <ATabPane key="logs" :tab="$gettext('Logs')" />
          <ATabPane key="config" :tab="$gettext('Configuration File')" />
        </ATabs>
      </div>

      <div class="site-title-row">
        <div class="site-title">
          {{ $gettext('Edit %{n}', { n: name }) }}
        </div>
        <ATag v-if="data.status === ConfigStatus.Enabled" color="blue">
          {{ $gettext('Enabled') }}
        </ATag>
        <ATag v-else-if="data.status === ConfigStatus.Disabled" color="red">
          {{ $gettext('Disabled') }}
        </ATag>
        <ATag v-else-if="data.status === ConfigStatus.Maintenance" color="orange">
          {{ $gettext('Maintenance') }}
        </ATag>
        <AButton v-if="filepath" type="link" class="ml-auto" @click="showHistory = true">
          <template #icon>
            <HistoryOutlined />
          </template>
          {{ $gettext('History') }}
        </AButton>
        <div class="mode-switch">
          <ASwitch
            size="small"
            :disabled="parseErrorStatus"
            :checked="advanceMode"
            :loading="loading"
            @change="handleModeChange"
          />
          <span>{{ advanceMode ? $gettext('Advance Mode') : $gettext('Basic Mode') }}</span>
        </div>
      </div>

      <InspectConfig ref="inspectConfig" class="mb-4" banner :namespace-id="data.namespace_id" />

      <div class="site-edit-content">
        <template v-if="activeKey === 'basic'">
          <BasicSettings />
        </template>

        <template v-else-if="activeKey === 'logs'">
          <AEmpty class="site-logs-placeholder" :description="$gettext('Log viewer will be available here.')" />
        </template>

        <template v-else>
          <div v-if="parseErrorStatus" class="mb-4">
            <AAlert
              banner
              :message="$gettext('Nginx Configuration Parse Error')"
              :description="parseErrorMessage"
              type="error"
              show-icon
            />
          </div>
          <CodeEditor
            :content="configText"
            no-border-radius
            @update:content="handleConfigContentUpdate"
          />
        </template>
      </div>
    </ACard>

    <FooterToolBar>
      <ASpace>
        <AButton @click="router.push('/sites/list')">
          {{ $gettext('Back') }}
        </AButton>
        <AButton type="primary" :loading="saving" @click="save">
          {{ $gettext('Save') }}
        </AButton>
      </ASpace>
    </FooterToolBar>

    <ConfigHistory
      v-model:visible="showHistory"
      v-model:current-content="configText"
      :filepath="filepath"
    />
  </div>
</template>

<style lang="less" scoped>
.site-edit-page {
  min-height: calc(100vh - 120px);
}

.site-edit-card {
  min-height: calc(100vh - 180px);

  :deep(.ant-card-body) {
    padding: 24px;
  }
}

.site-edit-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 18px;
}

.back-button {
  padding-left: 0;
  font-weight: 500;
}

.main-tabs {
  flex: 1;

  :deep(.ant-tabs-nav) {
    margin: 0;
  }

  :deep(.ant-tabs-content-holder) {
    display: none;
  }
}

.site-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
}

.site-title {
  font-size: 18px;
  font-weight: 600;
}

.mode-switch {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #6b7280;
}

.site-edit-content {
  min-height: 560px;
}

.site-logs-placeholder {
  padding: 80px 0;
}

@media (max-width: 768px) {
  .site-edit-header {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
