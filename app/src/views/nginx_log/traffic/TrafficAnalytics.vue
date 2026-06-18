<script setup lang="ts">
import type {
  AnalyticsRequest,
  ChinaMapData,
  DashboardAnalytics,
  DashboardExtraStats,
  DashboardRequest,
  NginxLogData,
  WorldMapData,
} from '@/api/nginx_log'
import { ReloadOutlined } from '@ant-design/icons-vue'
import dayjs from 'dayjs'
import nginxLog from '@/api/nginx_log'
import BrowserStatsTable from '../dashboard/components/BrowserStatsTable.vue'
import DailyTrendsChart from '../dashboard/components/DailyTrendsChart.vue'
import DeviceStatsTable from '../dashboard/components/DeviceStatsTable.vue'
import GeoMapChart from '../dashboard/components/GeoMapChart.vue'
import HourlyChart from '../dashboard/components/HourlyChart.vue'
import OSStatsTable from '../dashboard/components/OSStatsTable.vue'
import TopUrlsTable from '../dashboard/components/TopUrlsTable.vue'

interface MetricItem {
  label: string
  value: string
  suffix?: string
  danger?: boolean
  muted?: boolean
}

interface RankRow {
  label: string
  value: number
  percent: number
  color?: string
}

interface LogSourceOption {
  label: string
  value: string
}

const { message } = App.useApp()

const allLogsValue = '__all__'
const loading = ref(true)
const geoLoading = ref(false)
const logSourceLoading = ref(false)
const indexingEnabled = ref(false)
const dashboardData = ref<DashboardAnalytics | null>(null)
const worldMapData = ref<WorldMapData[] | null>(null)
const chinaMapData = ref<ChinaMapData[] | null>(null)
const accessLogs = ref<NginxLogData[]>([])
const selectedLogPath = ref(allLogsValue)
const timeRangePreset = ref('24h')
const dateRange = ref<[dayjs.Dayjs, dayjs.Dayjs]>([
  dayjs().subtract(24, 'hour'),
  dayjs(),
])

const rangeOptions = [
  { label: '近 24 小时', value: '24h' },
  { label: '近 7 天', value: '7d' },
  { label: '近 30 天', value: '30d' },
]

const refreshLoading = computed(() => loading.value || geoLoading.value)
const indexedAccessLogs = computed(() =>
  accessLogs.value.filter(log => log.path && ['indexed', 'ready'].includes(log.index_status ?? '')),
)
const selectedLogPaths = computed(() => {
  if (selectedLogPath.value === allLogsValue)
    return indexedAccessLogs.value.map(log => log.path).filter(Boolean) as string[]

  return selectedLogPath.value ? [selectedLogPath.value] : []
})
const currentLogPath = computed(() => selectedLogPath.value === allLogsValue ? '' : selectedLogPath.value)
const logSourceOptions = computed<LogSourceOption[]>(() => [
  {
    label: $gettext('All sites'),
    value: allLogsValue,
  },
  ...indexedAccessLogs.value.map(log => ({
    label: log.name || log.path || $gettext('Unnamed log'),
    value: log.path || '',
  })),
])

const emptyExtraStats: DashboardExtraStats = {
  top_ips: [],
  status_codes: [],
  methods: [],
  total_bytes: 0,
  avg_bytes: 0,
  request_time_count: 0,
  avg_request_time: 0,
  max_request_time: 0,
  upstream_time_count: 0,
  avg_upstream_time: 0,
  max_upstream_time: 0,
  client_error_count: 0,
  client_error_rate: 0,
  server_error_count: 0,
  server_error_rate: 0,
  blocked_ip_count: 0,
}

const inferredTotalUV = computed(() => {
  const dailyUVs = dashboardData.value?.daily_stats?.map(item => item.uv).filter(Boolean) ?? []
  return dailyUVs.length > 0 ? Math.max(...dailyUVs) : 0
})

const inferredAvgDailyUV = computed(() => {
  const dailyUVs = dashboardData.value?.daily_stats?.map(item => item.uv).filter(Boolean) ?? []
  if (dailyUVs.length === 0)
    return 0

  return dailyUVs.reduce((sum, value) => sum + value, 0) / dailyUVs.length
})

const normalizedDashboardData = computed<DashboardAnalytics | null>(() => {
  if (!dashboardData.value)
    return null

  const summary = {
    ...dashboardData.value.summary,
    total_uv: dashboardData.value.summary.total_uv || inferredTotalUV.value,
    avg_daily_uv: dashboardData.value.summary.avg_daily_uv || inferredAvgDailyUV.value,
  }

  return {
    ...dashboardData.value,
    extra_stats: dashboardData.value.extra_stats ?? emptyExtraStats,
    summary,
  }
})

const extraStats = computed(() => normalizedDashboardData.value?.extra_stats ?? emptyExtraStats)
const totalRequests = computed(() => normalizedDashboardData.value?.summary.total_pv ?? 0)
const totalVisitors = computed(() => normalizedDashboardData.value?.summary.total_uv ?? 0)
const totalPV = computed(() => totalRequests.value)
const uniqueIPCount = computed(() => totalVisitors.value)
const clientErrorCount = computed(() => extraStats.value.client_error_count ?? 0)
const clientErrorRate = computed(() => extraStats.value.client_error_rate ?? 0)
const serverErrorCount = computed(() => extraStats.value.server_error_count ?? 0)
const serverErrorRate = computed(() => extraStats.value.server_error_rate ?? 0)
const blockedCount = computed(() =>
  extraStats.value.status_codes
    ?.filter(item => item.status === 403)
    .reduce((sum, item) => sum + item.count, 0) ?? 0,
)
const attackIPCount = computed(() => extraStats.value.blocked_ip_count ?? 0)
const clientBlockedCount = computed(() => blockedCount.value)
const clientBlockedRate = computed(() => totalRequests.value > 0 ? (clientBlockedCount.value / totalRequests.value) * 100 : 0)

const overviewMetrics = computed<MetricItem[]>(() => [
  { label: '请求次数', value: formatCompact(totalRequests.value) },
  { label: '访问次数', value: formatCompact(totalPV.value) },
  { label: '独立访客', value: formatCompact(totalVisitors.value) },
  { label: '独立 IP', value: formatCompact(uniqueIPCount.value) },
  { label: '拦截次数', value: formatCompact(blockedCount.value), muted: true },
  { label: '攻击 IP', value: formatCompact(attackIPCount.value), muted: true },
  { label: '4xx 错误数', value: formatCompact(clientErrorCount.value), danger: clientErrorCount.value > 0 },
  { label: '4xx 错误率', value: formatPercent(clientErrorRate.value), danger: clientErrorRate.value > 0 },
  { label: '4xx 拦截数', value: formatCompact(clientBlockedCount.value), muted: true },
  { label: '4xx 拦截率', value: formatPercent(clientBlockedRate.value), muted: true },
  { label: '5xx 错误数', value: formatCompact(serverErrorCount.value), danger: serverErrorCount.value > 0 },
  { label: '5xx 错误率', value: formatPercent(serverErrorRate.value), danger: serverErrorRate.value > 0 },
])

const statusRows = computed(() => toRankRows(
  (extraStats.value.status_codes ?? []).map(item => ({ label: String(item.status), value: item.count })),
  '#ff7b55',
  6,
))

const methodRows = computed(() => toRankRows(
  (extraStats.value.methods ?? []).map(item => ({ label: item.method, value: item.count })),
  '#f3cbb0',
  5,
))

const statusDonutOuterStyle = computed(() => donutStyle(statusRows.value, ['#ff6b85', '#ff8557', '#6dbcf8', '#f4c9ab', '#ff9e57', '#8f99a6']))
const statusDonutInnerStyle = computed(() => donutStyle(methodRows.value, ['#ff8557', '#f4c9ab', '#6dbcf8', '#ef6378', '#8f99a6']))

async function loadIndexingStatus() {
  try {
    const status = await nginxLog.getAdvancedIndexingStatus()
    indexingEnabled.value = !!status.enabled
  }
  catch (error) {
    console.error('Failed to load advanced indexing status:', error)
    indexingEnabled.value = false
  }
}

async function loadLogSources() {
  if (!indexingEnabled.value) {
    accessLogs.value = []
    selectedLogPath.value = allLogsValue
    return
  }

  logSourceLoading.value = true
  try {
    const response = await nginxLog.list({
      type: 'access',
      indexed: 'true',
      sort_by: 'name',
      order: 'asc',
    })
    accessLogs.value = response.data ?? []

    if (selectedLogPath.value !== allLogsValue && !indexedAccessLogs.value.some(log => log.path === selectedLogPath.value))
      selectedLogPath.value = allLogsValue
  }
  catch (error) {
    console.error('Failed to load access log sources:', error)
    accessLogs.value = []
  }
  finally {
    logSourceLoading.value = false
  }
}

async function loadTimeRange() {
  if (!indexingEnabled.value)
    return

  try {
    const preflight = await nginxLog.getPreflight(currentLogPath.value)

    if (preflight.time_range?.end) {
      applyPresetRange(dayjs.unix(preflight.time_range.end))
    }
  }
  catch (error) {
    console.error('Failed to load traffic analytics time range:', error)
  }
}

async function loadDashboardData() {
  if (!indexingEnabled.value) {
    dashboardData.value = null
    loading.value = false
    return
  }

  loading.value = true
  try {
    const request: DashboardRequest = {
      log_path: currentLogPath.value,
      log_paths: selectedLogPaths.value,
      start_date: dateRange.value[0].format('YYYY-MM-DD'),
      end_date: dateRange.value[1].format('YYYY-MM-DD'),
    }

    dashboardData.value = await nginxLog.getDashboardAnalytics(request)
  }
  catch (error) {
    console.error('Failed to load traffic dashboard data:', error)
    dashboardData.value = null
    message.error($gettext('Failed to load traffic analytics data'))
  }
  finally {
    loading.value = false
  }
}

async function loadGeographicData() {
  if (!indexingEnabled.value) {
    worldMapData.value = null
    chinaMapData.value = null
    geoLoading.value = false
    return
  }

  geoLoading.value = true
  try {
    const request: AnalyticsRequest = {
      path: currentLogPath.value,
      log_paths: selectedLogPaths.value,
      start_time: dateRange.value[0].unix(),
      end_time: dateRange.value[1].unix(),
    }

    const [worldResponse, chinaResponse] = await Promise.all([
      nginxLog.getWorldMapData(request),
      nginxLog.getChinaMapData(request),
    ])

    worldMapData.value = worldResponse.data
    chinaMapData.value = chinaResponse.data
  }
  catch (error) {
    console.error('Failed to load traffic geographic data:', error)
    worldMapData.value = null
    chinaMapData.value = null
  }
  finally {
    geoLoading.value = false
  }
}

async function refreshAllData() {
  await loadIndexingStatus()

  if (!indexingEnabled.value) {
    loading.value = false
    geoLoading.value = false
    return
  }

  await Promise.all([
    loadDashboardData(),
    loadGeographicData(),
  ])
}

async function handleTimeRangeChange() {
  applyPresetRange(dayjs())
  await refreshAllData()
  message.success('流量分析已刷新')
}

async function handleLogSourceChange() {
  await loadTimeRange()
  await refreshAllData()
}

function applyPresetRange(endTime: dayjs.Dayjs) {
  if (timeRangePreset.value === '30d') {
    dateRange.value = [endTime.subtract(30, 'day').startOf('day'), endTime.endOf('day')]
    return
  }

  if (timeRangePreset.value === '7d') {
    dateRange.value = [endTime.subtract(7, 'day').startOf('day'), endTime.endOf('day')]
    return
  }

  dateRange.value = [endTime.subtract(24, 'hour'), endTime]
}

function formatCompact(value = 0) {
  if (value >= 1000000)
    return `${(value / 1000000).toFixed(1)}m`
  if (value >= 1000)
    return `${(value / 1000).toFixed(1)}k`
  return String(Math.round(value))
}

function formatPercent(value = 0) {
  return `${value.toFixed(2)}%`
}

function toRankRows(items: Array<{ label: string, value: number }>, color: string, limit = 5): RankRow[] {
  const sorted = items
    .filter(item => item.label && item.value > 0)
    .sort((a, b) => b.value - a.value)
    .slice(0, limit)
  const max = Math.max(...sorted.map(item => item.value), 1)

  return sorted.map(item => ({
    ...item,
    color,
    percent: Math.max(2, (item.value / max) * 100),
  }))
}

function donutStyle(rows: RankRow[], colors: string[]) {
  const total = rows.reduce((sum, row) => sum + row.value, 0)
  if (total === 0) {
    return {
      background: '#343434',
    }
  }

  let cursor = 0
  const segments = rows.map((row, index) => {
    const start = cursor
    const end = cursor + (row.value / total) * 100
    cursor = end
    const color = colors[index % colors.length]
    return `${color} ${start.toFixed(2)}% ${end.toFixed(2)}%`
  })

  return {
    background: `conic-gradient(${segments.join(', ')})`,
  }
}

function getStatusDotClass(row: RankRow) {
  if (Number(row.label) >= 500)
    return 'danger'
  if (Number(row.label) >= 400)
    return 'warning'
  if (Number(row.label) >= 300)
    return 'info'
  return 'success'
}

onMounted(async () => {
  await loadIndexingStatus()
  await loadLogSources()
  await loadTimeRange()
  await refreshAllData()
})
</script>

<template>
  <div class="traffic-analysis-page">
    <div class="traffic-header">
      <div>
        <div class="traffic-title">
          {{ $gettext('流量分析') }}
        </div>
        <div class="traffic-subtitle">
          {{ $gettext('基于高级索引的访问日志分析') }}
        </div>
      </div>
      <AFlex wrap="wrap" gap="small" align="center">
        <ASelect
          v-model:value="selectedLogPath"
          class="log-source-select"
          :loading="logSourceLoading"
          :options="logSourceOptions"
          show-search
          option-filter-prop="label"
          @change="handleLogSourceChange"
        />
        <ASelect
          v-model:value="timeRangePreset"
          class="range-select"
          :options="rangeOptions"
          @change="handleTimeRangeChange"
        />
        <AButton :loading="refreshLoading" @click="refreshAllData">
          <template #icon>
            <ReloadOutlined />
          </template>
          {{ $gettext('刷新') }}
        </AButton>
      </AFlex>
    </div>

    <AAlert
      v-if="!indexingEnabled"
      class="mb-4"
      show-icon
      type="warning"
      :message="$gettext('高级日志索引未启用')"
      :description="$gettext('流量分析需要访问日志索引。请在 Nginx 日志 > 日志列表中启用高级索引，以获取完整图表。')"
    />

    <ASpin :spinning="loading" :tip="$gettext('Loading data...')">
      <ACard :bordered="false" class="mb-4" :title="$gettext('流量概览')">
        <div class="metric-overview-grid">
          <div
            v-for="item in overviewMetrics"
            :key="item.label"
            class="metric-tile"
            :class="{ danger: item.danger, muted: item.muted }"
          >
            <div class="metric-title">
              {{ item.label }}
            </div>
            <div class="metric-value">
              {{ item.value }}
            </div>
          </div>
        </div>
      </ACard>

      <ARow :gutter="[16, 16]" class="mb-4">
        <ACol :span="24">
          <GeoMapChart
            :world-data="worldMapData"
            :china-data="chinaMapData"
            :loading="geoLoading"
            @refresh="refreshAllData"
          />
        </ACol>
      </ARow>

      <ARow :gutter="[16, 16]" class="mb-4">
        <ACol :xs="24" :xl="12">
          <HourlyChart
            :dashboard-data="normalizedDashboardData"
            :loading="loading"
            :end-date="dateRange[1].format('YYYY-MM-DD')"
          />
        </ACol>

        <ACol :xs="24" :xl="12">
          <DailyTrendsChart :dashboard-data="normalizedDashboardData" :loading="loading" />
        </ACol>
      </ARow>

      <ARow :gutter="[16, 16]" class="mb-4">
        <ACol :xs="24" :xl="8">
          <BrowserStatsTable :dashboard-data="normalizedDashboardData" :loading="loading" />
        </ACol>

        <ACol :xs="24" :xl="8">
          <OSStatsTable :dashboard-data="normalizedDashboardData" :loading="loading" />
        </ACol>

        <ACol :xs="24" :xl="8">
          <DeviceStatsTable :dashboard-data="normalizedDashboardData" :loading="loading" />
        </ACol>
      </ARow>

      <TopUrlsTable :dashboard-data="normalizedDashboardData" :loading="loading" />

      <ARow :gutter="[16, 16]" class="mb-4">
        <ACol :xs="24">
          <ACard :bordered="false" :title="$gettext('响应状态')">
            <div class="distribution-content status-distribution">
              <div class="double-donut">
                <div class="donut outer" :style="statusDonutOuterStyle" />
                <div class="donut inner" :style="statusDonutInnerStyle" />
              </div>
              <div class="legend-columns compact">
                <div class="legend-list">
                  <div v-for="row in statusRows" :key="row.label" class="legend-row">
                    <span class="legend-dot" :class="getStatusDotClass(row)" />
                    <span>{{ row.label }}</span>
                    <strong>{{ formatCompact(row.value) }}</strong>
                  </div>
                </div>
                <div class="legend-list">
                  <div v-for="row in methodRows" :key="row.label" class="legend-row">
                    <span class="legend-dot orange" />
                    <span>{{ row.label }}</span>
                    <strong>{{ formatCompact(row.value) }}</strong>
                  </div>
                </div>
              </div>
            </div>
          </ACard>
        </ACol>
      </ARow>
    </ASpin>
  </div>
</template>

<style lang="less" scoped>
.traffic-analysis-page {
  max-width: 100%;
  overflow-x: hidden;
}

.traffic-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin: 0 0 16px;
}

.traffic-title {
  color: var(--ant-color-text);
  font-size: 20px;
  font-weight: 600;
  line-height: 1.4;
}

.traffic-subtitle {
  margin-top: 4px;
  color: var(--ant-color-text-secondary);
  font-size: 13px;
}

.range-select {
  width: 132px;
}

.log-source-select {
  width: 260px;
}

.metric-overview-grid {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 16px;
}

.metric-tile {
  min-width: 0;
  border: 1px solid var(--ant-color-border-secondary);
  border-radius: 8px;
  padding: 14px 16px;
  background: var(--ant-color-fill-quaternary);
}

.metric-title {
  overflow: hidden;
  color: var(--ant-color-text-secondary);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.metric-value {
  margin-top: 8px;
  color: var(--ant-color-text);
  font-size: 24px;
  font-weight: 600;
  line-height: 1.25;
}

.metric-tile.danger {
  border-color: #ffccc7;

  .metric-value {
    color: #ff4d4f;
  }
}

.metric-tile.muted {
  .metric-value {
    color: var(--ant-color-text-tertiary);
  }
}

.distribution-content {
  display: grid;
  grid-template-columns: 132px minmax(0, 1fr);
  gap: 24px;
  align-items: center;
}

.double-donut {
  position: relative;
  width: 128px;
  height: 128px;
  border-radius: 50%;
}

.donut {
  position: absolute;
  border-radius: 50%;

  &::after {
    position: absolute;
    border-radius: 50%;
    background: var(--ant-color-bg-container);
    content: '';
  }
}

.donut.outer {
  inset: 0;

  &::after {
    inset: 22px;
  }
}

.donut.inner {
  inset: 34px;

  &::after {
    inset: 15px;
  }
}

.legend-columns {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;

  &.compact {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

.legend-list {
  display: grid;
  gap: 8px;
}

.legend-row {
  display: grid;
  grid-template-columns: 8px minmax(0, 1fr) auto;
  align-items: center;
  gap: 8px;
  color: var(--ant-color-text-secondary);
  font-size: 13px;

  span:nth-child(2) {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  strong {
    color: var(--ant-color-text);
    font-weight: 600;
  }
}

.legend-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--ant-color-primary);

  &.orange,
  &.warning {
    background: #fa8c16;
  }

  &.blue,
  &.info {
    background: #1677ff;
  }

  &.green,
  &.success {
    background: #52c41a;
  }

  &.danger {
    background: #ff4d4f;
  }
}

@media (max-width: 1200px) {
  .metric-overview-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .distribution-content,
  .legend-columns,
  .legend-columns.compact {
    grid-template-columns: 1fr;
  }

  .double-donut {
    justify-self: center;
  }
}

@media (max-width: 768px) {
  .traffic-header {
    align-items: stretch;
    flex-direction: column;
  }

  .metric-overview-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .log-source-select,
  .range-select {
    width: 100%;
  }

}

@media (max-width: 480px) {
  .metric-overview-grid {
    grid-template-columns: 1fr;
  }
}
</style>
