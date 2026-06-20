package reconciliation

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/adminplus/app/notifications"
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type RunInput struct {
	SupplierBills     []*adminplusdomain.SupplierBillLine
	LocalUsages       []*adminplusdomain.LocalUsageLine
	TimeTolerance     time.Duration
	CostMismatchCents int64
}

type RunResult struct {
	Lines   []*adminplusdomain.ReconciliationLine `json:"lines"`
	Summary adminplusdomain.ReconciliationSummary `json:"summary"`
}

type Notifier interface {
	NotifyReconciliationAnomaly(ctx context.Context, in RunInput, result *RunResult) error
}

type Service struct {
	notifier Notifier
}

func NewService() *Service {
	return NewServiceWithNotifier(nil)
}

func NewServiceWithNotifier(notifier Notifier) *Service {
	return &Service{notifier: notifier}
}

func (s *Service) Run(ctx context.Context, in RunInput) (*RunResult, error) {
	if len(in.SupplierBills) == 0 && len(in.LocalUsages) == 0 {
		return nil, badRequest("RECONCILIATION_INPUT_REQUIRED", "supplier bills or local usages are required")
	}
	if in.TimeTolerance <= 0 {
		in.TimeTolerance = time.Minute
	}
	if in.CostMismatchCents < 0 {
		return nil, badRequest("RECONCILIATION_COST_MISMATCH_INVALID", "cost mismatch threshold must be non-negative")
	}

	localIndexes := buildLocalIndexes(in.LocalUsages)
	usedLocal := make(map[int64]struct{})
	lines := make([]*adminplusdomain.ReconciliationLine, 0, len(in.SupplierBills)+len(in.LocalUsages))
	for _, bill := range in.SupplierBills {
		if bill == nil {
			continue
		}
		local := findBestLocalUsage(bill, localIndexes, usedLocal, in.TimeTolerance)
		if local == nil {
			lines = append(lines, supplierOnlyLine(bill))
			continue
		}
		usedLocal[local.ID] = struct{}{}
		lines = append(lines, matchedLine(bill, local, in.CostMismatchCents))
	}
	for _, usage := range in.LocalUsages {
		if usage == nil {
			continue
		}
		if _, ok := usedLocal[usage.ID]; ok {
			continue
		}
		lines = append(lines, localOnlyLine(usage))
	}
	result := &RunResult{
		Lines:   lines,
		Summary: summarize(lines, len(in.SupplierBills), len(in.LocalUsages)),
	}
	s.notifyReconciliationAnomaly(ctx, in, result)
	return result, nil
}

func (s *Service) notifyReconciliationAnomaly(ctx context.Context, in RunInput, result *RunResult) {
	if s == nil || s.notifier == nil || result == nil || !hasReconciliationAnomaly(result) {
		return
	}
	if err := s.notifier.NotifyReconciliationAnomaly(ctx, in, result); err != nil {
		slog.Warn("admin plus reconciliation notification failed", "err", err)
	}
}

type FeishuNotifier struct {
	sender *notifications.Feishu
}

func NewFeishuNotifierFromEnv(repo notifications.Repository) *FeishuNotifier {
	sender := notifications.NewFeishuFromEnv(repo)
	if sender == nil {
		return nil
	}
	return &FeishuNotifier{sender: sender}
}

func (n *FeishuNotifier) NotifyReconciliationAnomaly(ctx context.Context, in RunInput, result *RunResult) error {
	if n == nil || n.sender == nil || result == nil || !hasReconciliationAnomaly(result) {
		return nil
	}
	eventID, dedupeKey := buildReconciliationEventIdentity(in, result)
	return n.sender.SendEvent(ctx, notifications.Event{
		Type:           "reconciliation.anomaly",
		ID:             eventID,
		SupplierID:     singleSupplierID(in.SupplierBills),
		DedupeKey:      dedupeKey,
		ThrottleKey:    "anomaly",
		ThrottleWindow: notifications.DefaultThrottleWindow,
		Text:           buildFeishuReconciliationText(result),
	})
}

func hasReconciliationAnomaly(result *RunResult) bool {
	if result == nil {
		return false
	}
	if result.Summary.ProfitCents < 0 {
		return true
	}
	for _, line := range result.Lines {
		if line == nil {
			continue
		}
		if line.Status != adminplusdomain.ReconciliationStatusMatched {
			return true
		}
	}
	return false
}

func buildFeishuReconciliationText(result *RunResult) string {
	if result == nil {
		return ""
	}
	statusCounts := reconciliationStatusCounts(result.Lines)
	currency := firstLineCurrency(result.Lines)
	return fmt.Sprintf(
		"【Sub2API Admin Plus 对账异常通知】\n供应商账单：%d\n本地用量：%d\n匹配：%d\n供应商单边：%d\n本地单边：%d\n币种不一致：%d\n成本异常：%d\n成本：%s\n收入：%s\n利润：%s\n利润率：%s",
		result.Summary.TotalSupplierLines,
		result.Summary.TotalLocalLines,
		result.Summary.MatchedLines,
		statusCounts[adminplusdomain.ReconciliationStatusSupplierOnly],
		statusCounts[adminplusdomain.ReconciliationStatusLocalOnly],
		statusCounts[adminplusdomain.ReconciliationStatusCurrencyMismatch],
		statusCounts[adminplusdomain.ReconciliationStatusCostMismatch],
		formatReconCents(result.Summary.CostCents, currency),
		formatReconCents(result.Summary.RevenueCents, currency),
		formatReconCents(result.Summary.ProfitCents, currency),
		formatReconPercent(result.Summary.ProfitMargin),
	)
}

func buildReconciliationEventIdentity(in RunInput, result *RunResult) (int64, string) {
	digest := sha256.Sum256([]byte(reconciliationIdentitySeed(in, result)))
	value := int64(binary.BigEndian.Uint64(digest[:8]) & 0x7fffffffffffffff)
	if value == 0 {
		value = 1
	}
	return value, fmt.Sprintf("feishu:reconciliation.anomaly:%x", digest[:16])
}

func reconciliationIdentitySeed(in RunInput, result *RunResult) string {
	parts := make([]string, 0, len(in.SupplierBills)+len(in.LocalUsages)+len(result.Lines)+1)
	parts = append(parts, fmt.Sprintf("summary:%d:%d:%d:%d:%d:%d",
		result.Summary.TotalSupplierLines,
		result.Summary.TotalLocalLines,
		result.Summary.CostCents,
		result.Summary.RevenueCents,
		result.Summary.ProfitCents,
		result.Summary.MatchedLines,
	))
	for _, bill := range in.SupplierBills {
		if bill == nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("bill:%d:%d:%s:%s:%s:%d:%s",
			bill.ID,
			bill.SupplierID,
			normalizeKey(bill.ExternalRequestID),
			normalizeKey(bill.Model),
			normalizeCurrency(bill.Currency),
			bill.CostCents,
			bill.StartedAt.UTC().Format(time.RFC3339Nano),
		))
	}
	for _, usage := range in.LocalUsages {
		if usage == nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("usage:%d:%s:%s:%s:%d:%s",
			usage.ID,
			normalizeKey(usage.ExternalRequestID),
			normalizeKey(usage.Model),
			normalizeCurrency(usage.Currency),
			usage.RevenueCents,
			usage.StartedAt.UTC().Format(time.RFC3339Nano),
		))
	}
	for _, line := range result.Lines {
		if line == nil || line.Status == adminplusdomain.ReconciliationStatusMatched {
			continue
		}
		parts = append(parts, fmt.Sprintf("line:%s:%d:%d:%s:%s:%d:%d",
			line.Status,
			line.SupplierBillID,
			line.LocalUsageID,
			normalizeKey(line.ExternalRequestID),
			normalizeKey(line.Model),
			line.CostCents,
			line.RevenueCents,
		))
	}
	sort.Strings(parts)
	return strings.Join(parts, "|")
}

func reconciliationStatusCounts(lines []*adminplusdomain.ReconciliationLine) map[adminplusdomain.ReconciliationStatus]int64 {
	counts := make(map[adminplusdomain.ReconciliationStatus]int64)
	for _, line := range lines {
		if line == nil {
			continue
		}
		counts[line.Status]++
	}
	return counts
}

func singleSupplierID(bills []*adminplusdomain.SupplierBillLine) int64 {
	var supplierID int64
	for _, bill := range bills {
		if bill == nil || bill.SupplierID <= 0 {
			continue
		}
		if supplierID == 0 {
			supplierID = bill.SupplierID
			continue
		}
		if supplierID != bill.SupplierID {
			return 0
		}
	}
	return supplierID
}

func firstLineCurrency(lines []*adminplusdomain.ReconciliationLine) string {
	for _, line := range lines {
		if line == nil {
			continue
		}
		return normalizeCurrency(line.Currency)
	}
	return "USD"
}

func formatReconCents(cents int64, currency string) string {
	sign := ""
	value := cents
	if value < 0 {
		sign = "-"
		value = -value
	}
	return fmt.Sprintf("%s%.2f %s", sign, float64(value)/100, normalizeCurrency(currency))
}

func formatReconPercent(value *float64) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%.2f%%", *value*100)
}

type localIndexes struct {
	byRequestID map[string][]*adminplusdomain.LocalUsageLine
	byModel     map[string][]*adminplusdomain.LocalUsageLine
}

func buildLocalIndexes(usages []*adminplusdomain.LocalUsageLine) localIndexes {
	indexes := localIndexes{
		byRequestID: make(map[string][]*adminplusdomain.LocalUsageLine),
		byModel:     make(map[string][]*adminplusdomain.LocalUsageLine),
	}
	for _, usage := range usages {
		if usage == nil {
			continue
		}
		if key := normalizeKey(usage.ExternalRequestID); key != "" {
			indexes.byRequestID[key] = append(indexes.byRequestID[key], usage)
		}
		if model := normalizeKey(usage.Model); model != "" {
			indexes.byModel[model] = append(indexes.byModel[model], usage)
		}
	}
	return indexes
}

func findBestLocalUsage(bill *adminplusdomain.SupplierBillLine, indexes localIndexes, used map[int64]struct{}, tolerance time.Duration) *adminplusdomain.LocalUsageLine {
	if key := normalizeKey(bill.ExternalRequestID); key != "" {
		if usage := firstUnused(indexes.byRequestID[key], used); usage != nil {
			return usage
		}
	}
	candidates := indexes.byModel[normalizeKey(bill.Model)]
	var best *adminplusdomain.LocalUsageLine
	var bestDelta time.Duration
	for _, usage := range candidates {
		if usage == nil {
			continue
		}
		if _, ok := used[usage.ID]; ok {
			continue
		}
		delta := usage.StartedAt.Sub(bill.StartedAt)
		if delta < 0 {
			delta = -delta
		}
		if delta > tolerance {
			continue
		}
		if best == nil || delta < bestDelta {
			best = usage
			bestDelta = delta
		}
	}
	return best
}

func firstUnused(items []*adminplusdomain.LocalUsageLine, used map[int64]struct{}) *adminplusdomain.LocalUsageLine {
	for _, item := range items {
		if item == nil {
			continue
		}
		if _, ok := used[item.ID]; ok {
			continue
		}
		return item
	}
	return nil
}

func matchedLine(bill *adminplusdomain.SupplierBillLine, usage *adminplusdomain.LocalUsageLine, mismatchCents int64) *adminplusdomain.ReconciliationLine {
	status := adminplusdomain.ReconciliationStatusMatched
	notes := ""
	currency := normalizeCurrency(bill.Currency)
	if normalizeCurrency(usage.Currency) != currency {
		status = adminplusdomain.ReconciliationStatusCurrencyMismatch
		notes = "currency mismatch"
	} else if usage.RevenueCents < bill.CostCents {
		status = adminplusdomain.ReconciliationStatusCostMismatch
		notes = "revenue is below supplier cost"
	} else if mismatchCents > 0 && absInt64(usage.RevenueCents-bill.CostCents) <= mismatchCents {
		status = adminplusdomain.ReconciliationStatusCostMismatch
		notes = "revenue is too close to supplier cost"
	}
	profit := usage.RevenueCents - bill.CostCents
	return &adminplusdomain.ReconciliationLine{
		Status:            status,
		SupplierBillID:    bill.ID,
		LocalUsageID:      usage.ID,
		ExternalRequestID: firstNonEmpty(bill.ExternalRequestID, usage.ExternalRequestID),
		Model:             firstNonEmpty(bill.Model, usage.Model),
		Currency:          currency,
		CostCents:         bill.CostCents,
		RevenueCents:      usage.RevenueCents,
		ProfitCents:       profit,
		ProfitMargin:      profitMargin(profit, usage.RevenueCents),
		Notes:             notes,
	}
}

func supplierOnlyLine(bill *adminplusdomain.SupplierBillLine) *adminplusdomain.ReconciliationLine {
	return &adminplusdomain.ReconciliationLine{
		Status:            adminplusdomain.ReconciliationStatusSupplierOnly,
		SupplierBillID:    bill.ID,
		ExternalRequestID: bill.ExternalRequestID,
		Model:             bill.Model,
		Currency:          normalizeCurrency(bill.Currency),
		CostCents:         bill.CostCents,
		RevenueCents:      0,
		ProfitCents:       -bill.CostCents,
		ProfitMargin:      nil,
		Notes:             "supplier bill has no matching local usage",
	}
}

func localOnlyLine(usage *adminplusdomain.LocalUsageLine) *adminplusdomain.ReconciliationLine {
	return &adminplusdomain.ReconciliationLine{
		Status:            adminplusdomain.ReconciliationStatusLocalOnly,
		LocalUsageID:      usage.ID,
		ExternalRequestID: usage.ExternalRequestID,
		Model:             usage.Model,
		Currency:          normalizeCurrency(usage.Currency),
		CostCents:         0,
		RevenueCents:      usage.RevenueCents,
		ProfitCents:       usage.RevenueCents,
		ProfitMargin:      profitMargin(usage.RevenueCents, usage.RevenueCents),
		Notes:             "local usage has no matching supplier bill",
	}
}

func summarize(lines []*adminplusdomain.ReconciliationLine, supplierCount int, localCount int) adminplusdomain.ReconciliationSummary {
	var summary adminplusdomain.ReconciliationSummary
	summary.TotalSupplierLines = int64(supplierCount)
	summary.TotalLocalLines = int64(localCount)
	for _, line := range lines {
		if line == nil {
			continue
		}
		switch line.Status {
		case adminplusdomain.ReconciliationStatusMatched:
			summary.MatchedLines++
		case adminplusdomain.ReconciliationStatusSupplierOnly:
			summary.SupplierOnlyLines++
		case adminplusdomain.ReconciliationStatusLocalOnly:
			summary.LocalOnlyLines++
		}
		summary.CostCents += line.CostCents
		summary.RevenueCents += line.RevenueCents
		summary.ProfitCents += line.ProfitCents
	}
	summary.ProfitMargin = profitMargin(summary.ProfitCents, summary.RevenueCents)
	return summary
}

func normalizeKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeCurrency(value string) string {
	v := strings.ToUpper(strings.TrimSpace(value))
	if len(v) != 3 {
		return "USD"
	}
	return v
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func profitMargin(profitCents int64, revenueCents int64) *float64 {
	if revenueCents == 0 {
		return nil
	}
	value := float64(profitCents) / float64(revenueCents)
	return &value
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

func badRequest(reason string, message string) error {
	return infraerrors.New(http.StatusBadRequest, reason, message)
}
