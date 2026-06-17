<script setup lang="ts">
import type { EChartsOption } from 'echarts'
import type {
  AnalyticsRequest,
  ChinaMapData,
  DashboardAnalytics,
  DashboardExtraStats,
  DashboardRequest,
  HourlyStats,
  NginxLogData,
  WorldMapData,
} from '@/api/nginx_log'
import { ReloadOutlined } from '@ant-design/icons-vue'
import dayjs from 'dayjs'
import { LineChart, MapChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, VisualMapComponent } from 'echarts/components'
import { registerMap, use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import countries from 'i18n-iso-countries'
import en from 'i18n-iso-countries/langs/en.json'
import { storeToRefs } from 'pinia'
import VChart from 'vue-echarts'
import nginxLog from '@/api/nginx_log'
import { useSettingsStore } from '@/pinia'
import china from '../dashboard/components/ChinaMapChart/china.json'
import world from '../dashboard/components/WorldMapChart/world.json'

use([MapChart, LineChart, GridComponent, TooltipComponent, VisualMapComponent, CanvasRenderer])
countries.registerLocale(en)
registerMap('traffic-world', world as unknown as Parameters<typeof registerMap>[1])
registerMap('traffic-china', china as unknown as Parameters<typeof registerMap>[1])

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
const settings = useSettingsStore()
const { theme } = storeToRefs(settings)

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
const geoRegion = ref<'world' | 'china'>('world')
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
const effectiveWorldMapData = computed(() => worldMapData.value ?? [])
const effectiveChinaMapData = computed(() => chinaMapData.value ?? [])
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

const hourlyTrendItems = computed<HourlyStats[]>(() => {
  const hourly = normalizedDashboardData.value?.hourly_stats ?? []
  if (hourly.length > 0)
    return pickRecentActiveItems(hourly, 24)

  return []
})

const hourlyValues = computed(() => {
  const values = hourlyTrendItems.value.map(item => item.pv)
  if (values.length > 0)
    return values

  return normalizedDashboardData.value?.daily_stats?.map(item => item.pv).slice(-24) ?? []
})

const visitPeak = computed(() => Math.max(...hourlyValues.value, 0))

const hourlyTrendOption = computed<EChartsOption>(() => {
  const isDark = theme.value === 'dark'
  const lineColor = isDark ? '#69b1ff' : '#1677ff'
  const areaColor = isDark ? 'rgba(105, 177, 255, 0.18)' : 'rgba(22, 119, 255, 0.14)'
  const gridLineColor = isDark ? 'rgba(255, 255, 255, 0.08)' : 'rgba(5, 5, 5, 0.06)'
  const axisTextColor = isDark ? 'rgba(255, 255, 255, 0.55)' : 'rgba(0, 0, 0, 0.45)'
  const data = hourlyValues.value.length > 0 ? hourlyValues.value : Array.from({ length: 24 }).fill(0) as number[]
  const labels = hourlyTrendItems.value.length > 0
    ? hourlyTrendItems.value.map(item => `${String(item.hour).padStart(2, '0')}:00`)
    : data.map((_, index) => `${String(index).padStart(2, '0')}:00`)

  return {
    animationDuration: 450,
    grid: {
      top: 20,
      right: 16,
      bottom: 34,
      left: 42,
    },
    tooltip: {
      trigger: 'axis',
      backgroundColor: isDark ? '#1f1f1f' : '#fff',
      borderColor: isDark ? '#303030' : '#f0f0f0',
      textStyle: {
        color: isDark ? '#f5f5f5' : '#262626',
      },
      axisPointer: {
        type: 'line',
        lineStyle: {
          color: lineColor,
          opacity: 0.3,
        },
      },
      formatter: params => {
        const [item] = params as Array<{ axisValue?: string, value?: number }>
        return `${item?.axisValue ?? ''}<br/>请求数：${formatCompact(Number(item?.value ?? 0))}`
      },
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: labels,
      axisTick: { show: false },
      axisLine: { show: false },
      axisLabel: {
        color: axisTextColor,
        interval: Math.max(Math.floor(labels.length / 6), 0),
      },
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
      splitLine: {
        lineStyle: {
          color: gridLineColor,
        },
      },
      axisLabel: {
        color: axisTextColor,
        formatter: value => formatCompact(Number(value)),
      },
    },
    series: [{
      type: 'line',
      name: '请求数',
      data,
      smooth: true,
      showSymbol: false,
      symbolSize: 6,
      lineStyle: {
        width: 3,
        color: lineColor,
      },
      areaStyle: {
        color: areaColor,
      },
      emphasis: {
        focus: 'series',
        showSymbol: true,
      },
    }],
  }
})

const geoRows = computed(() => {
  const worldRows = effectiveWorldMapData.value
    .map(item => ({ label: formatGeoName(item), value: item.value || 0 }))
    .filter(item => item.value > 0)

  const chinaRows = effectiveChinaMapData.value
    .map(item => ({ label: item.name, value: item.value || 0 }))
    .filter(item => item.value > 0)

  const rows = geoRegion.value === 'china'
    ? (chinaRows.length > 0 ? chinaRows : worldRows)
    : (worldRows.length > 0 ? worldRows : chinaRows)

  return toRankRows(rows, '#f3cbb0', 7)
})

const geoMapOption = computed<EChartsOption>(() => {
  const sourceData = geoRegion.value === 'china'
    ? effectiveChinaMapData.value
    : effectiveWorldMapData.value

  const maxValue = Math.max(...sourceData.map(item => item.value || 0), 1)
  const topValue = Math.max(...sourceData.map(item => item.value || 0), 0)
  const isDark = theme.value === 'dark'
  const mapName = geoRegion.value === 'china' ? 'traffic-china' : 'traffic-world'
  const chartData = geoRegion.value === 'china'
    ? effectiveChinaMapData.value.map(item => ({
        name: item.name,
        value: item.value,
        localizedName: item.name,
        percent: item.percent,
        itemStyle: {
          areaColor: item.value === topValue ? '#ffc107' : '#f2c7a6',
        },
      }))
    : effectiveWorldMapData.value.map(item => {
        const englishName = countries.getName(item.code, 'en', { select: 'alias' }) || item.code
        const localizedName = formatGeoName(item)

        return {
          name: englishName,
          value: item.value,
          localizedName,
          percent: item.percent,
          itemStyle: {
            areaColor: item.value === topValue ? '#ffc107' : '#f2c7a6',
          },
        }
      })

  return {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item',
      backgroundColor: '#fff',
      borderColor: '#ff7a45',
      borderWidth: 1,
      padding: [10, 14],
      textStyle: {
        color: '#666',
        fontSize: 14,
      },
      formatter: params => {
        const data = params.data as { localizedName?: string, value?: number, percent?: number } | undefined
        if (!data || data.value === undefined)
          return `${params.name}: ${$gettext('No data')}`

        return `
          <div style="min-width: 120px;">
            <div style="font-size: 16px; color: #555; margin-bottom: 6px;">${data.localizedName || params.name}</div>
            <div style="font-size: 15px;">${$gettext('Visits')}: ${Number(data.value || 0).toLocaleString()}</div>
          </div>
        `
      },
    },
    visualMap: {
      show: false,
      min: 0,
      max: maxValue,
      inRange: {
        color: ['#3f3f3f', '#f2c7a6', '#ffc107'],
      },
    },
    series: [{
      type: 'map',
      map: mapName,
      roam: false,
      zoom: geoRegion.value === 'china' ? 1.12 : 1.16,
      selectedMode: false,
      label: {
        show: true,
        color: '#111',
        fontSize: 18,
        formatter: params => {
          const data = params.data as { localizedName?: string, value?: number } | undefined
          if (!data || data.value !== topValue)
            return ''

          return data.localizedName || params.name
        },
      },
      emphasis: {
        disabled: false,
        label: {
          show: true,
          color: '#111',
          fontSize: 18,
        },
        itemStyle: {
          areaColor: '#ffc107',
          borderColor: '#f0f0f0',
          borderWidth: 0.8,
        },
      },
      itemStyle: {
        areaColor: isDark ? '#454545' : '#d9d9d9',
        borderColor: isDark ? '#6a6a6a' : '#bfbfbf',
        borderWidth: 0.45,
      },
      data: chartData,
    }],
  }
})

const osRows = computed(() => toRankRows(
  (normalizedDashboardData.value?.operating_systems ?? []).map(item => ({ label: item.os || 'Unknown', value: item.count })),
  '#ff8a57',
  5,
))

const browserRows = computed(() => toRankRows(
  (normalizedDashboardData.value?.browsers ?? []).map(item => ({ label: item.browser || 'Unknown', value: item.count })),
  '#f4c9ab',
  5,
))

const deviceRows = computed(() => toRankRows(
  (normalizedDashboardData.value?.devices ?? []).map(item => ({ label: item.device || 'Unknown', value: item.count })),
  '#71bdf7',
  5,
))

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

const visitedPageRows = computed(() => toRankRows(
  (normalizedDashboardData.value?.top_urls ?? []).map(item => ({ label: item.url, value: item.visits })),
  '#ef5d83',
  5,
))

const clientDonutOuterStyle = computed(() => donutStyle(osRows.value, ['#ff8557', '#f7c7aa', '#ef6378', '#61b8f6', '#ff9e57']))
const clientDonutInnerStyle = computed(() => donutStyle(browserRows.value, ['#f4c9ab', '#65b9f7', '#ef6378', '#ff9e57', '#8f99a6']))
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

function formatGeoName(item: WorldMapData) {
  const countryNames: Record<string, string> = {
    CN: '中国',
    US: '美国',
    JP: '日本',
    DE: '德国',
    FR: '法国',
    IN: '印度',
    ID: '印尼',
    BR: '巴西',
    RU: '俄罗斯',
  }

  return item.region || item.province || item.city || countryNames[item.code] || item.code || 'Unknown'
}

function pickRecentActiveItems(items: HourlyStats[], limit: number) {
  const lastActiveIndex = items.findLastIndex(item => item.pv > 0)
  const picked = lastActiveIndex === -1
    ? items.slice(-limit)
    : items.slice(Math.max(lastActiveIndex - limit + 1, 0), lastActiveIndex + 1)

  if (picked.length >= limit)
    return picked

  const firstTimestamp = picked[0]?.timestamp ?? Math.floor(Date.now() / 1000)
  const padding = Array.from({ length: limit - picked.length }, (_, index) => {
    const timestamp = firstTimestamp - (limit - picked.length - index) * 3600
    return {
      hour: dayjs.unix(timestamp).hour(),
      pv: 0,
      timestamp,
      uv: 0,
    }
  })

  return [...padding, ...picked]
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

function setGeoRegion(value: 'world' | 'china') {
  geoRegion.value = value
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
        <ACol :xs="24" :xl="16">
          <ACard :bordered="false" title="地理分布">
            <template #extra>
              <AFlex wrap="wrap" gap="small">
                <ASegmented
                  v-model:value="geoRegion"
                  :options="[
                    { label: '全球', value: 'world' },
                    { label: '中国', value: 'china' },
                  ]"
                  @change="value => setGeoRegion(value as 'world' | 'china')"
                />
              </AFlex>
            </template>
            <div class="geo-layout">
              <div class="flat-map-panel">
                <VChart
                  class="traffic-map-chart"
                  :option="geoMapOption"
                  autoresize
                />
              </div>
              <div class="geo-rank-panel">
                <AEmpty v-if="geoRows.length === 0" description="暂无数据" />
                <div v-for="row in geoRows" v-else :key="row.label" class="rank-row">
                  <div class="rank-meta">
                    <span>{{ row.label }}</span>
                    <strong>{{ formatCompact(row.value) }}</strong>
                  </div>
                  <AProgress :percent="Math.round(row.percent)" size="small" :show-info="false" />
                </div>
              </div>
            </div>
          </ACard>
        </ACol>

        <ACol :xs="24" :xl="8">
          <ACard :bordered="false" class="h-full" title="小时请求趋势">
            <template #extra>
              <span class="text-gray-500 text-sm">峰值 {{ formatCompact(visitPeak) }}</span>
            </template>
            <VChart
              class="hourly-trend-chart"
              :option="hourlyTrendOption"
              autoresize
            />
          </ACard>
        </ACol>
      </ARow>

      <ARow :gutter="[16, 16]" class="mb-4">
        <ACol :xs="24" :xl="12">
          <ACard :bordered="false" :title="$gettext('客户端分布')">
            <div class="distribution-content">
              <div class="double-donut">
                <div class="donut outer" :style="clientDonutOuterStyle" />
                <div class="donut inner" :style="clientDonutInnerStyle" />
              </div>
              <div class="legend-columns">
                <div class="legend-list">
                  <div v-for="row in osRows" :key="row.label" class="legend-row">
                    <span class="legend-dot orange" />
                    <span>{{ row.label }}</span>
                    <strong>{{ formatCompact(row.value) }}</strong>
                  </div>
                </div>
                <div class="legend-list">
                  <div v-for="row in browserRows" :key="row.label" class="legend-row">
                    <span class="legend-dot blue" />
                    <span>{{ row.label }}</span>
                    <strong>{{ formatCompact(row.value) }}</strong>
                  </div>
                </div>
                <div class="legend-list">
                  <div v-for="row in deviceRows" :key="row.label" class="legend-row">
                    <span class="legend-dot green" />
                    <span>{{ row.label }}</span>
                    <strong>{{ formatCompact(row.value) }}</strong>
                  </div>
                </div>
              </div>
            </div>
          </ACard>
        </ACol>

        <ACol :xs="24" :xl="12">
          <ACard :bordered="false" :title="$gettext('响应状态')">
            <div class="distribution-content">
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

      <ACard :bordered="false" :title="$gettext('热门 URL')">
        <AEmpty v-if="visitedPageRows.length === 0" :description="$gettext('暂无数据')" />
        <div v-for="row in visitedPageRows" v-else :key="row.label" class="url-row">
          <div class="rank-meta">
            <span>{{ row.label }}</span>
            <strong>{{ formatCompact(row.value) }}</strong>
          </div>
          <AProgress :percent="Math.round(row.percent)" size="small" :show-info="false" status="active" />
        </div>
      </ACard>
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

.geo-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 300px;
  gap: 24px;
  align-items: center;
  min-height: 430px;
}

.flat-map-panel {
  display: grid;
  place-items: center;
  min-width: 0;
  height: 430px;
  overflow: hidden;
  border: 1px solid var(--ant-color-border-secondary);
  border-radius: 8px;
  background: var(--ant-color-fill-quaternary);
}

.traffic-map-chart {
  width: 100%;
  height: 430px;
}

.geo-rank-panel {
  display: grid;
  gap: 16px;
  border: 1px solid var(--ant-color-border-secondary);
  border-radius: 8px;
  padding: 18px;
  background: var(--ant-color-fill-quaternary);
}

.rank-row,
.url-row {
  display: grid;
  gap: 8px;
}

.url-row + .url-row {
  margin-top: 14px;
}

.rank-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  color: var(--ant-color-text-secondary);
  font-size: 13px;

  span {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  strong {
    color: var(--ant-color-text);
    font-weight: 600;
  }
}

.hourly-trend-chart {
  width: 100%;
  height: 312px;
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

  .geo-layout,
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

  .flat-map-panel,
  .traffic-map-chart {
    height: 300px;
  }
}

@media (max-width: 480px) {
  .metric-overview-grid {
    grid-template-columns: 1fr;
  }
}
</style>
