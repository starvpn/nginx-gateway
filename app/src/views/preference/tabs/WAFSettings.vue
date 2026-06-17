<script setup lang="ts">
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data } = storeToRefs(systemSettingsStore)

const modeOptions = computed(() => [
  {
    label: $gettext('Observe only (SIMULATE)'),
    value: 'SIMULATE',
  },
  {
    label: $gettext('Block attacks (ACTIVE)'),
    value: 'ACTIVE',
  },
  {
    label: $gettext('Loaded but inactive (INACTIVE)'),
    value: 'INACTIVE',
  },
])
</script>

<template>
  <AForm layout="vertical">
    <AAlert
      class="mb-4"
      show-icon
      type="warning"
      :message="$gettext('OpenResty WAF')"
      :description="$gettext('This switch controls the global lua-resty-waf policy. Start with observe-only mode to check false positives before enabling blocking mode.')"
    />

    <AFormItem :label="$gettext('Enabled')">
      <ASwitch v-model:checked="data.waf.enabled" />
    </AFormItem>

    <AFormItem :label="$gettext('Mode')">
      <ASelect
        v-model:value="data.waf.mode"
        :disabled="!data.waf.enabled"
        :options="modeOptions"
      />
      <div class="text-secondary mt-1">
        {{ $gettext('SIMULATE records WAF events without blocking. ACTIVE returns the configured deny status for blocked requests.') }}
      </div>
    </AFormItem>

    <AFormItem :label="$gettext('Score Threshold')">
      <AInputNumber
        v-model:value="data.waf.score_threshold"
        :min="1"
        :max="100"
        :disabled="!data.waf.enabled"
      />
    </AFormItem>

    <AFormItem :label="$gettext('Deny Status')">
      <AInputNumber
        v-model:value="data.waf.deny_status"
        :min="400"
        :max="599"
        :disabled="!data.waf.enabled"
      />
    </AFormItem>

    <AFormItem :label="$gettext('Log altered requests only')">
      <ASwitch
        v-model:checked="data.waf.event_log_altered_only"
        :disabled="!data.waf.enabled"
      />
    </AFormItem>

    <AFormItem :label="$gettext('Debug')">
      <ASwitch
        v-model:checked="data.waf.debug"
        :disabled="!data.waf.enabled"
      />
    </AFormItem>

    <AFormItem :label="$gettext('Runtime config file')">
      <p>{{ data.waf.config_path }}</p>
    </AFormItem>
  </AForm>
</template>

<style lang="less" scoped>
</style>
