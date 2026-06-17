package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/searcher"
	"github.com/uozi-tech/cosy/logger"
)

// GetDashboardAnalytics generates comprehensive dashboard analytics
func (s *service) GetDashboardAnalytics(ctx context.Context, req *DashboardQueryRequest) (*DashboardAnalytics, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	if err := s.ValidateTimeRange(req.StartTime, req.EndTime); err != nil {
		return nil, fmt.Errorf("invalid time range: %w", err)
	}

	searchReq := &searcher.SearchRequest{
		StartTime:      &req.StartTime,
		EndTime:        &req.EndTime,
		LogPaths:       req.LogPaths,
		UseMainLogPath: true, // Use main_log_path field for efficient log group queries
		IncludeFacets:  true,
		FacetFields:    []string{"browser", "os", "device_type"}, // Removed 'ip' to reduce facet computation
		FacetSize:      50,                                       // Significantly reduced for faster facet computation
		UseCache:       true,
		SortBy:         "timestamp",
		SortOrder:      "desc",
		Limit:          0, // Don't fetch documents, use aggregations instead
	}

	// Execute search
	result, err := s.searcher.Search(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("failed to search logs for dashboard: %w", err)
	}

	// DEBUG: Check if documents have main_log_path field
	if result.TotalHits == 0 {
		logger.Warnf("⚠️ No results found with main_log_path query!")
		debugReq := &searcher.SearchRequest{
			Limit:    3,
			UseCache: false,
			Fields:   []string{"main_log_path", "file_path", "timestamp"},
		}
		if debugResult, debugErr := s.searcher.Search(ctx, debugReq); debugErr == nil {
			logger.Warnf("📊 Index contains %d total documents", debugResult.TotalHits)
			if len(debugResult.Hits) > 0 {
				for i, hit := range debugResult.Hits {
					logger.Warnf("📄 Document %d fields: %+v", i, hit.Fields)
					if i >= 2 {
						break
					}
				}
			}
		}
	}

	// --- DIAGNOSTIC LOGGING ---
	logger.Debugf("Dashboard search completed. Total Hits: %d, Returned Hits: %d, Facets: %d",
		result.TotalHits, len(result.Hits), len(result.Facets))
	if result.TotalHits > uint64(len(result.Hits)) {
		logger.Warnf("Dashboard sampling: using %d/%d documents for time calculations (%.1f%% coverage)",
			len(result.Hits), result.TotalHits, float64(len(result.Hits))/float64(result.TotalHits)*100)
	}
	// --- END DIAGNOSTIC LOGGING ---

	// Initialize analytics with empty slices
	analytics := &DashboardAnalytics{}

	// Calculate analytics if we have results
	if result.TotalHits > 0 {
		// For now, use batch queries to get complete data
		analytics.HourlyStats = s.calculateHourlyStatsWithBatch(ctx, req)
		analytics.DailyStats = s.calculateDailyStatsWithBatch(ctx, req)

		// Use cardinality counter for efficient unique URLs counting
		analytics.TopURLs = s.calculateTopURLsWithCardinality(ctx, req)

		analytics.Browsers = s.calculateBrowserStats(result)
		analytics.OperatingSystems = s.calculateOSStats(result)
		analytics.Devices = s.calculateDeviceStats(result)
		analytics.ExtraStats = s.calculateDashboardExtraStatsWithBatch(ctx, req)
	} else {
		// Ensure slices are initialized even if there are no hits
		analytics.HourlyStats = make([]HourlyAccessStats, 0)
		analytics.DailyStats = make([]DailyAccessStats, 0)
		analytics.TopURLs = make([]URLAccessStats, 0)
		analytics.Browsers = make([]BrowserAccessStats, 0)
		analytics.OperatingSystems = make([]OSAccessStats, 0)
		analytics.Devices = make([]DeviceAccessStats, 0)
		analytics.ExtraStats = emptyDashboardExtraStats()
	}

	// Calculate summary with cardinality counting for accurate unique pages
	analytics.Summary = s.calculateDashboardSummaryWithCardinality(ctx, analytics, result, req)

	return analytics, nil
}

// calculateHourlyStats calculates hourly access statistics.
// Returns 48 hours of data centered around the end_date to support all timezones.
func (s *service) calculateHourlyStats(result *searcher.SearchResult, startTime, endTime int64) []HourlyAccessStats {
	// Use a map with timestamp as key for easier processing
	hourlyMap := make(map[int64]*HourlyAccessStats)
	uniqueIPsPerHour := make(map[int64]map[string]bool)

	// Calculate 48 hours range: from UTC end_date minus 12 hours to plus 36 hours
	// This covers UTC-12 to UTC+14 timezones
	endDate := time.Unix(endTime, 0).UTC()
	endDateStart := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, time.UTC)

	// Create hourly buckets for 48 hours (12 hours before to 36 hours after the UTC date boundary)
	rangeStart := endDateStart.Add(-12 * time.Hour)
	rangeEnd := endDateStart.Add(36 * time.Hour)

	// Initialize hourly buckets
	for t := rangeStart; t.Before(rangeEnd); t = t.Add(time.Hour) {
		timestamp := t.Unix()
		hourlyMap[timestamp] = &HourlyAccessStats{
			Hour:      t.Hour(),
			UV:        0,
			PV:        0,
			Timestamp: timestamp,
		}
		uniqueIPsPerHour[timestamp] = make(map[string]bool)
	}

	// Process search results - count hits within the 48-hour window
	for _, hit := range result.Hits {
		if timestampField, ok := hit.Fields["timestamp"]; ok {
			if timestampFloat, ok := timestampField.(float64); ok {
				timestamp := int64(timestampFloat)

				// Check if this hit falls within our 48-hour window
				if timestamp >= rangeStart.Unix() && timestamp < rangeEnd.Unix() {
					// Round down to the hour
					t := time.Unix(timestamp, 0).UTC()
					hourTimestamp := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, time.UTC).Unix()

					if stats, exists := hourlyMap[hourTimestamp]; exists {
						stats.PV++
						if ipField, ok := hit.Fields["ip"]; ok {
							if ip, ok := ipField.(string); ok && ip != "" {
								if !uniqueIPsPerHour[hourTimestamp][ip] {
									uniqueIPsPerHour[hourTimestamp][ip] = true
									stats.UV++
								}
							}
						}
					}
				}
			}
		}
	}

	// Convert to slice and sort by timestamp
	var stats []HourlyAccessStats
	for _, stat := range hourlyMap {
		stats = append(stats, *stat)
	}

	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Timestamp < stats[j].Timestamp
	})

	return stats
}

// calculateDailyStats calculates daily access statistics
func (s *service) calculateDailyStats(result *searcher.SearchResult, startTime, endTime int64) []DailyAccessStats {
	dailyMap := make(map[string]*DailyAccessStats)
	uniqueIPsPerDay := make(map[string]map[string]bool)

	// Initialize daily buckets for the entire time range
	start := time.Unix(startTime, 0)
	end := time.Unix(endTime, 0)
	for t := start; t.Before(end) || t.Equal(end); t = t.AddDate(0, 0, 1) {
		dateStr := t.Format("2006-01-02")
		if _, exists := dailyMap[dateStr]; !exists {
			dailyMap[dateStr] = &DailyAccessStats{
				Date:      dateStr,
				UV:        0,
				PV:        0,
				Timestamp: t.Unix(),
			}
			uniqueIPsPerDay[dateStr] = make(map[string]bool)
		}
	}

	// Process search results
	for _, hit := range result.Hits {
		if timestampField, ok := hit.Fields["timestamp"]; ok {
			if timestampFloat, ok := timestampField.(float64); ok {
				timestamp := int64(timestampFloat)
				t := time.Unix(timestamp, 0)
				dateStr := t.Format("2006-01-02")

				if stats, exists := dailyMap[dateStr]; exists {
					stats.PV++
					if ipField, ok := hit.Fields["ip"]; ok {
						if ip, ok := ipField.(string); ok && ip != "" {
							if !uniqueIPsPerDay[dateStr][ip] {
								uniqueIPsPerDay[dateStr][ip] = true
								stats.UV++
							}
						}
					}
				}
			}
		}
	}

	// Convert to slice and sort
	var stats []DailyAccessStats
	for _, stat := range dailyMap {
		stats = append(stats, *stat)
	}

	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Timestamp < stats[j].Timestamp
	})

	return stats
}

// calculateTopURLs calculates top URL statistics from facets (legacy method)
func (s *service) calculateTopURLs(result *searcher.SearchResult) []URLAccessStats {
	if facet, ok := result.Facets["path_exact"]; ok {
		logger.Infof("📊 Facet-based URL calculation: facet.Total=%d, TotalHits=%d",
			facet.Total, result.TotalHits)

		urlStats := calculateTopFieldStats(facet, int(result.TotalHits), func(term string, count int, percent float64) URLAccessStats {
			return URLAccessStats{URL: term, Visits: count, Percent: percent}
		})

		logger.Infof("📈 Calculated %d URL stats from facet", len(urlStats))
		return urlStats
	} else {
		logger.Errorf("❌ path_exact facet not found in search results")
		return []URLAccessStats{}
	}
}

// calculateTopURLsWithCardinality calculates top URL statistics using facet-based approach
// Always returns actual top URLs with their visit counts instead of just a summary
func (s *service) calculateTopURLsWithCardinality(ctx context.Context, req *DashboardQueryRequest) []URLAccessStats {
	// Always use facet-based calculation to get actual top URLs with visit counts
	searchReq := &searcher.SearchRequest{
		StartTime:      &req.StartTime,
		EndTime:        &req.EndTime,
		LogPaths:       req.LogPaths,
		UseMainLogPath: true, // Use main_log_path for efficient log group queries
		IncludeFacets:  true,
		FacetFields:    []string{"path_exact"},
		FacetSize:      100, // Reasonable facet size to get top URLs
		UseCache:       true,
	}

	result, err := s.searcher.Search(ctx, searchReq)
	if err != nil {
		logger.Errorf("Failed to search for URL facets: %v", err)
		return []URLAccessStats{}
	}

	// Get actual top URLs with visit counts
	return s.calculateTopURLs(result)
}

// calculateBrowserStats calculates browser statistics from facets
func (s *service) calculateBrowserStats(result *searcher.SearchResult) []BrowserAccessStats {
	return calculateTopFieldStats(result.Facets["browser"], int(result.TotalHits), func(term string, count int, percent float64) BrowserAccessStats {
		return BrowserAccessStats{Browser: term, Count: count, Percent: percent}
	})
}

// calculateOSStats calculates operating system statistics from facets
func (s *service) calculateOSStats(result *searcher.SearchResult) []OSAccessStats {
	return calculateTopFieldStats(result.Facets["os"], int(result.TotalHits), func(term string, count int, percent float64) OSAccessStats {
		return OSAccessStats{OS: term, Count: count, Percent: percent}
	})
}

// calculateDeviceStats calculates device statistics from facets
func (s *service) calculateDeviceStats(result *searcher.SearchResult) []DeviceAccessStats {
	return calculateTopFieldStats(result.Facets["device_type"], int(result.TotalHits), func(term string, count int, percent float64) DeviceAccessStats {
		return DeviceAccessStats{Device: term, Count: count, Percent: percent}
	})
}

func emptyDashboardExtraStats() DashboardExtraStats {
	return DashboardExtraStats{
		TopIPs:      make([]IPAccessStats, 0),
		StatusCodes: make([]StatusAccessStats, 0),
		Methods:     make([]MethodAccessStats, 0),
	}
}

func (s *service) calculateDashboardExtraStatsWithBatch(ctx context.Context, req *DashboardQueryRequest) DashboardExtraStats {
	extra := emptyDashboardExtraStats()

	batchSize := 150000
	offset := 0
	ipCounts := make(map[string]int)
	blockedIPs := make(map[string]struct{})
	statusCounts := make(map[int]int)
	methodCounts := make(map[string]int)

	var processed int
	var requestTimeTotal, requestTimeMax float64
	var upstreamTimeTotal, upstreamTimeMax float64

	for {
		searchReq := &searcher.SearchRequest{
			StartTime:      &req.StartTime,
			EndTime:        &req.EndTime,
			LogPaths:       req.LogPaths,
			UseMainLogPath: true,
			Limit:          batchSize,
			Offset:         offset,
			Fields: []string{
				"ip",
				"status",
				"method",
				"bytes_sent",
				"request_time",
				"upstream_time",
			},
			UseCache: false,
		}

		result, err := s.searcher.Search(ctx, searchReq)
		if err != nil {
			logger.Errorf("Failed to fetch dashboard extra stats batch at offset %d: %v", offset, err)
			break
		}

		for _, hit := range result.Hits {
			processed++

			ip := getStringField(hit.Fields, "ip")
			if ip != "" {
				ipCounts[ip]++
			}

			if status, ok := getIntField(hit.Fields, "status"); ok {
				statusCounts[status]++
				if status == 403 && ip != "" {
					blockedIPs[ip] = struct{}{}
				}
				if status >= 400 && status < 500 {
					extra.ClientErrorCount++
				} else if status >= 500 && status < 600 {
					extra.ServerErrorCount++
				}
			}

			if method := getStringField(hit.Fields, "method"); method != "" {
				methodCounts[method]++
			}

			if bytesSent, ok := getInt64Field(hit.Fields, "bytes_sent"); ok {
				extra.TotalBytes += bytesSent
			}

			if requestTime, ok := getFloatField(hit.Fields, "request_time"); ok {
				extra.RequestTimeCount++
				requestTimeTotal += requestTime
				if requestTime > requestTimeMax {
					requestTimeMax = requestTime
				}
			}

			if upstreamTime, ok := getFloatField(hit.Fields, "upstream_time"); ok {
				extra.UpstreamTimeCount++
				upstreamTimeTotal += upstreamTime
				if upstreamTime > upstreamTimeMax {
					upstreamTimeMax = upstreamTime
				}
			}
		}

		if len(result.Hits) < batchSize {
			break
		}

		offset += batchSize
	}

	if processed == 0 {
		return extra
	}

	extra.TopIPs = buildTopIPStats(ipCounts, processed, 10)
	extra.StatusCodes = buildStatusStats(statusCounts, processed)
	extra.Methods = buildMethodStats(methodCounts, processed)
	extra.BlockedIPCount = len(blockedIPs)
	extra.AvgBytes = float64(extra.TotalBytes) / float64(processed)
	extra.ClientErrorRate = float64(extra.ClientErrorCount) / float64(processed) * 100
	extra.ServerErrorRate = float64(extra.ServerErrorCount) / float64(processed) * 100

	if extra.RequestTimeCount > 0 {
		extra.AvgRequestTime = requestTimeTotal / float64(extra.RequestTimeCount)
		extra.MaxRequestTime = requestTimeMax
	}

	if extra.UpstreamTimeCount > 0 {
		extra.AvgUpstreamTime = upstreamTimeTotal / float64(extra.UpstreamTimeCount)
		extra.MaxUpstreamTime = upstreamTimeMax
	}

	return extra
}

func buildTopIPStats(counts map[string]int, total int, limit int) []IPAccessStats {
	items := make([]IPAccessStats, 0, len(counts))
	for ip, count := range counts {
		items = append(items, IPAccessStats{
			IP:      ip,
			Count:   count,
			Percent: percentOf(count, total),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].IP < items[j].IP
		}
		return items[i].Count > items[j].Count
	})

	if limit > 0 && len(items) > limit {
		return items[:limit]
	}

	return items
}

func buildStatusStats(counts map[int]int, total int) []StatusAccessStats {
	items := make([]StatusAccessStats, 0, len(counts))
	for status, count := range counts {
		items = append(items, StatusAccessStats{
			Status:  status,
			Count:   count,
			Percent: percentOf(count, total),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Status < items[j].Status
	})

	return items
}

func buildMethodStats(counts map[string]int, total int) []MethodAccessStats {
	items := make([]MethodAccessStats, 0, len(counts))
	for method, count := range counts {
		items = append(items, MethodAccessStats{
			Method:  method,
			Count:   count,
			Percent: percentOf(count, total),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Method < items[j].Method
		}
		return items[i].Count > items[j].Count
	})

	return items
}

func percentOf(count, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(count) / float64(total) * 100
}

func getStringField(fields map[string]interface{}, key string) string {
	value, ok := fields[key]
	if !ok || value == nil {
		return ""
	}

	if stringValue, ok := value.(string); ok {
		return stringValue
	}

	return fmt.Sprint(value)
}

func getIntField(fields map[string]interface{}, key string) (int, bool) {
	value, ok := fields[key]
	if !ok || value == nil {
		return 0, false
	}

	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		return int(parsed), err == nil
	case string:
		parsed, err := strconv.Atoi(typed)
		return parsed, err == nil
	default:
		parsed, err := strconv.Atoi(fmt.Sprint(value))
		return parsed, err == nil
	}
}

func getInt64Field(fields map[string]interface{}, key string) (int64, bool) {
	value, ok := fields[key]
	if !ok || value == nil {
		return 0, false
	}

	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case float64:
		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		return parsed, err == nil
	default:
		parsed, err := strconv.ParseInt(fmt.Sprint(value), 10, 64)
		return parsed, err == nil
	}
}

func getFloatField(fields map[string]interface{}, key string) (float64, bool) {
	value, ok := fields[key]
	if !ok || value == nil {
		return 0, false
	}

	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		if typed == "-" || typed == "" {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(typed, 64)
		return parsed, err == nil
	default:
		parsed, err := strconv.ParseFloat(fmt.Sprint(value), 64)
		return parsed, err == nil
	}
}

// calculateTopFieldStats is a generic function to calculate top N items from a facet result.
func calculateTopFieldStats[T any](
	facet *searcher.Facet,
	totalHits int,
	creator func(term string, count int, percent float64) T,
) []T {
	if facet == nil || totalHits == 0 {
		return []T{}
	}

	var items []T
	for _, term := range facet.Terms {
		percent := float64(term.Count) / float64(totalHits) * 100
		items = append(items, creator(term.Term, term.Count, percent))
	}
	return items
}

// calculateDashboardSummary calculates summary statistics
func (s *service) calculateDashboardSummary(analytics *DashboardAnalytics, result *searcher.SearchResult) DashboardSummary {
	// Calculate total UV from IP facet, which is now reliable.
	totalUV := 0
	if result.Facets != nil {
		if ipFacet, ok := result.Facets["ip"]; ok {
			// The total number of unique terms in the facet is the UV count.
			totalUV = ipFacet.Total
		}
	}

	totalPV := int(result.TotalHits)

	// Calculate average daily UV and PV
	var avgDailyUV, avgDailyPV float64
	if len(analytics.DailyStats) > 0 {
		var sumPV int
		for _, daily := range analytics.DailyStats {
			sumPV += daily.PV
		}
		// Use total unique visitors divided by number of days for accurate daily UV average
		// The totalUV represents unique visitors across the entire period, not sum of daily UVs
		avgDailyUV = float64(totalUV) / float64(len(analytics.DailyStats))
		avgDailyPV = float64(sumPV) / float64(len(analytics.DailyStats))
	}

	// Find peak hour
	var peakHour, peakHourTraffic int
	for _, hourly := range analytics.HourlyStats {
		if hourly.PV > peakHourTraffic {
			peakHour = hourly.Hour
			peakHourTraffic = hourly.PV
		}
	}

	return DashboardSummary{
		TotalUV:         totalUV,
		TotalPV:         totalPV,
		AvgDailyUV:      avgDailyUV,
		AvgDailyPV:      avgDailyPV,
		PeakHour:        peakHour,
		PeakHourTraffic: peakHourTraffic,
	}
}

// calculateDashboardSummaryWithCardinality calculates enhanced summary statistics using cardinality counters
func (s *service) calculateDashboardSummaryWithCardinality(ctx context.Context, analytics *DashboardAnalytics, result *searcher.SearchResult, req *DashboardQueryRequest) DashboardSummary {
	// Start with the basic summary but we'll override the UV calculation
	summary := s.calculateDashboardSummary(analytics, result)

	// Use cardinality counter for accurate unique visitor (UV) counting if available
	cardinalityCounter := s.getCardinalityCounter()
	if cardinalityCounter != nil {
		// Count unique IPs (visitors) using cardinality counter instead of limited facet
		uvCardReq := &searcher.CardinalityRequest{
			Field:          "ip",
			StartTime:      &req.StartTime,
			EndTime:        &req.EndTime,
			LogPaths:       req.LogPaths,
			UseMainLogPath: true, // Use main_log_path for efficient log group queries
		}

		if uvResult, err := cardinalityCounter.Count(ctx, uvCardReq); err == nil {
			// Override the facet-limited UV count with accurate cardinality count
			summary.TotalUV = int(uvResult.Cardinality)

			// Recalculate average daily UV with accurate count
			if len(analytics.DailyStats) > 0 {
				summary.AvgDailyUV = float64(summary.TotalUV) / float64(len(analytics.DailyStats))
			}

			// Log the improvement - handle case where IP facet might not exist
			facetUV := "N/A"
			if result.Facets != nil && result.Facets["ip"] != nil {
				facetUV = fmt.Sprintf("%d", result.Facets["ip"].Total)
			}
			logger.Infof("✓ Accurate UV count using Counter: %d (was limited to %s by facet)",
				uvResult.Cardinality, facetUV)
		} else {
			logger.Errorf("Failed to count unique visitors with cardinality counter: %v", err)
		}

		// Also count unique pages for additional insights
		pageCardReq := &searcher.CardinalityRequest{
			Field:          "path_exact",
			StartTime:      &req.StartTime,
			EndTime:        &req.EndTime,
			LogPaths:       req.LogPaths,
			UseMainLogPath: true, // Use main_log_path for efficient log group queries
		}

		if pageResult, err := cardinalityCounter.Count(ctx, pageCardReq); err == nil {
			logger.Debugf("Accurate unique pages count: %d (vs Total PV: %d)", pageResult.Cardinality, summary.TotalPV)

			if pageResult.Cardinality <= uint64(summary.TotalPV) {
				logger.Infof("✓ Unique pages (%d) ≤ Total PV (%d) - data consistency verified", pageResult.Cardinality, summary.TotalPV)
			} else {
				logger.Warnf("⚠ Unique pages (%d) > Total PV (%d) - possible data inconsistency", pageResult.Cardinality, summary.TotalPV)
			}
		} else {
			logger.Errorf("Failed to count unique pages: %v", err)
		}
	} else {
		logger.Warnf("Counter not available, UV count limited by facet size to %d", summary.TotalUV)
	}

	return summary
}

// calculateDailyStatsWithBatch calculates daily statistics by fetching data in batches
func (s *service) calculateDailyStatsWithBatch(ctx context.Context, req *DashboardQueryRequest) []DailyAccessStats {
	dailyMap := make(map[string]*DailyAccessStats)
	uniqueIPsPerDay := make(map[string]map[string]bool)

	// Initialize daily buckets for the entire time range
	start := time.Unix(req.StartTime, 0)
	end := time.Unix(req.EndTime, 0)
	for t := start; t.Before(end) || t.Equal(end); t = t.AddDate(0, 0, 1) {
		dateStr := t.Format("2006-01-02")
		if _, exists := dailyMap[dateStr]; !exists {
			dailyMap[dateStr] = &DailyAccessStats{
				Date:      dateStr,
				UV:        0,
				PV:        0,
				Timestamp: t.Unix(),
			}
			uniqueIPsPerDay[dateStr] = make(map[string]bool)
		}
	}

	// Process data in batches to avoid memory issues - significantly increased batch size for maximum performance
	batchSize := 150000 // Increased batch size for better throughput
	offset := 0

	logger.Debugf("📅 Daily stats batch query: start=%d (%s), end=%d (%s), expected days=%d",
		req.StartTime, time.Unix(req.StartTime, 0).Format("2006-01-02 15:04:05"),
		req.EndTime, time.Unix(req.EndTime, 0).Format("2006-01-02 15:04:05"),
		len(dailyMap))

	totalProcessedDaily := 0
	for {
		searchReq := &searcher.SearchRequest{
			StartTime:      &req.StartTime,
			EndTime:        &req.EndTime,
			LogPaths:       req.LogPaths,
			UseMainLogPath: true, // Use main_log_path for efficient log group queries
			Limit:          batchSize,
			Offset:         offset,
			Fields:         []string{"timestamp", "ip"},
			UseCache:       false, // Don't cache intermediate results
		}

		result, err := s.searcher.Search(ctx, searchReq)
		if err != nil {
			logger.Errorf("Failed to fetch batch at offset %d: %v", offset, err)
			break
		}

		logger.Debugf("🔍 Daily batch %d: returned %d hits, totalHits=%d",
			offset/batchSize, len(result.Hits), result.TotalHits)

		// Process this batch of results
		processedInBatch := 0
		for _, hit := range result.Hits {
			if timestampField, ok := hit.Fields["timestamp"]; ok {
				if timestampFloat, ok := timestampField.(float64); ok {
					timestamp := int64(timestampFloat)
					t := time.Unix(timestamp, 0)
					dateStr := t.Format("2006-01-02")

					if stats, exists := dailyMap[dateStr]; exists {
						stats.PV++
						processedInBatch++
						if ipField, ok := hit.Fields["ip"]; ok {
							if ip, ok := ipField.(string); ok && ip != "" {
								if !uniqueIPsPerDay[dateStr][ip] {
									uniqueIPsPerDay[dateStr][ip] = true
									stats.UV++
								}
							}
						}
					} else {
						if offset < 10 { // Only log first few mismatches to avoid spam
							logger.Debugf("⚠️  Daily: timestamp %d (%s) -> date %s not found in dailyMap",
								timestamp, t.Format("2006-01-02 15:04:05"), dateStr)
						}
					}
				} else {
					if offset < 10 {
						logger.Debugf("⚠️  Daily: timestamp field is not float64: %T = %v", timestampField, timestampField)
					}
				}
			} else {
				if offset < 10 {
					logger.Debugf("⚠️  Daily: no timestamp field in hit: %+v", hit.Fields)
				}
			}
		}

		logger.Debugf("📝 Daily batch %d: processed %d/%d records", offset/batchSize, processedInBatch, len(result.Hits))

		// Check if we've processed all results
		if len(result.Hits) < batchSize {
			break
		}

		offset += batchSize
		totalProcessedDaily += processedInBatch

		// Log progress
		logger.Debugf("Processed %d/%d records for daily stats", offset, result.TotalHits)
	}

	logger.Infof("📊 Daily stats processing completed: %d total records processed, %d day buckets", totalProcessedDaily, len(dailyMap))

	// Convert to slice and sort
	var stats []DailyAccessStats
	for _, stat := range dailyMap {
		stats = append(stats, *stat)
	}

	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Timestamp < stats[j].Timestamp
	})

	return stats
}

// calculateHourlyStatsWithBatch calculates hourly statistics by fetching data in batches
func (s *service) calculateHourlyStatsWithBatch(ctx context.Context, req *DashboardQueryRequest) []HourlyAccessStats {
	// Use a map with timestamp as key for easier processing
	hourlyMap := make(map[int64]*HourlyAccessStats)
	uniqueIPsPerHour := make(map[int64]map[string]bool)

	// For user date range queries, cover the full requested range plus timezone buffer
	// This ensures we capture data in all timezones for the requested dates
	startDate := time.Unix(req.StartTime, 0).UTC()
	endDate := time.Unix(req.EndTime, 0).UTC()

	// Add timezone buffer: 12 hours before start, 12 hours after end
	// This covers UTC-12 to UTC+12 timezones adequately
	rangeStart := startDate.Add(-12 * time.Hour)
	rangeEnd := endDate.Add(12 * time.Hour)

	// Initialize hourly buckets
	for t := rangeStart; t.Before(rangeEnd); t = t.Add(time.Hour) {
		timestamp := t.Unix()
		hourlyMap[timestamp] = &HourlyAccessStats{
			Hour:      t.Hour(),
			UV:        0,
			PV:        0,
			Timestamp: timestamp,
		}
		uniqueIPsPerHour[timestamp] = make(map[string]bool)
	}

	// Process data in batches - significantly increased batch size for maximum performance
	batchSize := 150000 // Increased batch size for better throughput
	offset := 0

	// Adjust time range for hourly query
	hourlyStartTime := rangeStart.Unix()
	hourlyEndTime := rangeEnd.Unix()

	logger.Debugf("🕐 Hourly stats batch query: start=%d (%s), end=%d (%s), expected buckets=%d",
		hourlyStartTime, time.Unix(hourlyStartTime, 0).Format("2006-01-02 15:04:05"),
		hourlyEndTime, time.Unix(hourlyEndTime, 0).Format("2006-01-02 15:04:05"),
		len(hourlyMap))

	totalProcessed := 0
	for {
		searchReq := &searcher.SearchRequest{
			StartTime:      &hourlyStartTime,
			EndTime:        &hourlyEndTime,
			LogPaths:       req.LogPaths,
			UseMainLogPath: true, // Use main_log_path for efficient log group queries
			Limit:          batchSize,
			Offset:         offset,
			Fields:         []string{"timestamp", "ip"},
			UseCache:       false,
		}

		result, err := s.searcher.Search(ctx, searchReq)
		if err != nil {
			logger.Errorf("Failed to fetch batch at offset %d: %v", offset, err)
			break
		}

		logger.Debugf("🔍 Hourly batch %d: returned %d hits, totalHits=%d",
			offset/batchSize, len(result.Hits), result.TotalHits)

		// Process this batch of results
		processedInBatch := 0
		for _, hit := range result.Hits {
			if timestampField, ok := hit.Fields["timestamp"]; ok {
				if timestampFloat, ok := timestampField.(float64); ok {
					timestamp := int64(timestampFloat)

					// Round down to the hour
					t := time.Unix(timestamp, 0).UTC()
					hourTimestamp := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, time.UTC).Unix()

					if stats, exists := hourlyMap[hourTimestamp]; exists {
						stats.PV++
						processedInBatch++
						if ipField, ok := hit.Fields["ip"]; ok {
							if ip, ok := ipField.(string); ok && ip != "" {
								if !uniqueIPsPerHour[hourTimestamp][ip] {
									uniqueIPsPerHour[hourTimestamp][ip] = true
									stats.UV++
								}
							}
						}
					} else {
						if offset < 10 { // Only log first few mismatches
							hourStr := time.Unix(hourTimestamp, 0).Format("2006-01-02 15:04:05")
							logger.Debugf("⚠️  Hourly: timestamp %d (%s) -> hour %d (%s) not found in hourlyMap",
								timestamp, t.Format("2006-01-02 15:04:05"), hourTimestamp, hourStr)
						}
					}
				} else {
					if offset < 10 {
						logger.Debugf("⚠️  Hourly: timestamp field is not float64: %T = %v", timestampField, timestampField)
					}
				}
			} else {
				if offset < 10 {
					logger.Debugf("⚠️  Hourly: no timestamp field in hit: %+v", hit.Fields)
				}
			}
		}

		logger.Debugf("📝 Hourly batch %d: processed %d/%d records", offset/batchSize, processedInBatch, len(result.Hits))

		// Check if we've processed all results
		if len(result.Hits) < batchSize {
			break
		}

		offset += batchSize

		totalProcessed += processedInBatch
		// Log progress
		logger.Debugf("Processed %d/%d records for hourly stats", offset, result.TotalHits)
	}

	logger.Infof("📊 Hourly stats processing completed: %d total records processed, %d hour buckets", totalProcessed, len(hourlyMap))

	// Convert to slice and sort by timestamp
	var stats []HourlyAccessStats
	for _, stat := range hourlyMap {
		stats = append(stats, *stat)
	}

	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Timestamp < stats[j].Timestamp
	})

	return stats
}
