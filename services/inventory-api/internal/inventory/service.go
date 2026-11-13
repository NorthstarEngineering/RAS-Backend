package inventory

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const (
	recentMovementsLimit  = 5
	consumptionWindowDays = 30
	maxCoverDays          = 3650
	maxTextLen            = 120
	maxLocationLen        = 40
	uncategorized         = "UNCATEGORIZED"
	unknownUnit           = "UNKNOWN"
)

// maxAmount is the largest value NUMERIC(12,2) can hold.
var maxAmount = decimal.RequireFromString("9999999999.99")

type Service struct {
	store   Store
	catalog GoodsCatalog
	users   UserDirectory
	log     *slog.Logger
	now     func() time.Time
}

// NewService wires the service. catalog / users / log may be nil (placeholders are used).
func NewService(store Store, catalog GoodsCatalog, users UserDirectory, log *slog.Logger) *Service {
	if catalog == nil {
		catalog = NoopCatalog{}
	}
	if users == nil {
		users = NoopUserDirectory{}
	}
	if log == nil {
		log = slog.Default()
	}
	return &Service{store: store, catalog: catalog, users: users, log: log, now: time.Now}
}

// ==================== Enrichment (catalog + user names) ====================

func uniqueIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// lookupGoods never fails the request: if the catalog is down, responses just
// come back without the good details.
func (s *Service) lookupGoods(ctx context.Context, ids []uuid.UUID) map[uuid.UUID]GoodInfo {
	if len(ids) == 0 {
		return map[uuid.UUID]GoodInfo{}
	}
	goods, err := s.catalog.GetGoods(ctx, uniqueIDs(ids))
	if err != nil {
		s.log.WarnContext(ctx, "catalog lookup failed", "err", err)
		return map[uuid.UUID]GoodInfo{}
	}
	if goods == nil {
		return map[uuid.UUID]GoodInfo{}
	}
	return goods
}

func (s *Service) lookupUserNames(ctx context.Context, ids []uuid.UUID) map[uuid.UUID]string {
	if len(ids) == 0 {
		return map[uuid.UUID]string{}
	}
	names, err := s.users.GetUserNames(ctx, uniqueIDs(ids))
	if err != nil {
		s.log.WarnContext(ctx, "user directory lookup failed", "err", err)
		return map[uuid.UUID]string{}
	}
	if names == nil {
		return map[uuid.UUID]string{}
	}
	return names
}

func goodSummary(goods map[uuid.UUID]GoodInfo, id uuid.UUID) *GoodSummary {
	info, ok := goods[id]
	if !ok {
		return nil
	}
	return &GoodSummary{
		Name:     info.Name,
		SKU:      info.SKU,
		Category: info.Category,
		Unit:     info.Unit,
		Barcode:  info.Barcode,
	}
}

func (s *Service) enrichInventories(ctx context.Context, items []InventoryResponse) {
	ids := make([]uuid.UUID, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.GoodID)
	}
	goods := s.lookupGoods(ctx, ids)
	for i := range items {
		items[i].Good = goodSummary(goods, items[i].GoodID)
	}
}

func (s *Service) enrichMovements(ctx context.Context, items []MovementResponse) {
	goodIDs := make([]uuid.UUID, 0, len(items))
	userIDs := make([]uuid.UUID, 0, len(items))
	for _, it := range items {
		goodIDs = append(goodIDs, it.GoodID)
		userIDs = append(userIDs, it.CreatedBy)
	}
	goods := s.lookupGoods(ctx, goodIDs)
	names := s.lookupUserNames(ctx, userIDs)
	for i := range items {
		items[i].Good = goodSummary(goods, items[i].GoodID)
		if n, ok := names[items[i].CreatedBy]; ok {
			name := n
			items[i].CreatedByName = &name
		}
	}
}

func goodIDsOf(items []Inventory) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(items))
	for _, inv := range items {
		ids = append(ids, inv.GoodID)
	}
	return ids
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// ==================== Writes ====================

// RecordMovement records a receipt (IN), issue/transfer-out (OUT) or
// adjustment (ADJUSTMENT) atomically: lock balance → check → compute → update → ledger.
func (s *Service) RecordMovement(ctx context.Context, actor Actor, req CreateMovementRequest) (CreateMovementResponse, error) {
	if err := validateActor(actor); err != nil {
		return CreateMovementResponse{}, err
	}

	req.Normalize()
	if err := validateMovementRequest(req); err != nil {
		return CreateMovementResponse{}, err
	}

	if req.Type == TypeAdjustment && !actor.Role.CanAdjust() {
		return CreateMovementResponse{}, ErrAdjustmentNotAllowed
	}

	branchID, err := resolveBranch(actor, req.BranchID)
	if err != nil {
		return CreateMovementResponse{}, err
	}

	if req.DestinationBranchID != nil && *req.DestinationBranchID == branchID {
		return CreateMovementResponse{}, ErrInvalidDestination
	}

	var (
		updated  Inventory
		movement InventoryMovement
	)

	err = s.store.WithTx(ctx, func(tx TxStore) error {
		current, err := tx.LockOrCreateInventory(ctx, branchID, req.GoodID)
		if err != nil {
			return err
		}

		// Optimistic check: the caller saw a balance on screen; refuse if it moved.
		if req.ExpectedQuantityBefore != nil && !req.ExpectedQuantityBefore.Equal(current.Quantity) {
			return &StaleBalanceError{Expected: *req.ExpectedQuantityBefore, Actual: current.Quantity}
		}

		newQty, recorded, err := applyMovement(current.Quantity, req)
		if err != nil {
			return err
		}

		if req.Type == TypeAdjustment && req.AdjustmentKind != nil {
			if err := checkAdjustmentKind(*req.AdjustmentKind, newQty.Sub(current.Quantity)); err != nil {
				return err
			}
		}

		// Unit cost changes only on receipts that carry a cost.
		newCost := current.UnitCost
		if req.Type == TypeIn && req.UnitCost != nil {
			newCost = *req.UnitCost
		}

		inv, err := tx.UpdateInventory(ctx, current.ID, newQty, newCost)
		if err != nil {
			return err
		}

		seq, err := tx.NextReference(ctx)
		if err != nil {
			return err
		}
		ref := formatReference(req.Type, req.DestinationBranchID != nil, s.now(), seq)

		// Snapshot of the unit cost at movement time (financial impact of the movement).
		costSnapshot := newCost

		m := InventoryMovement{
			ID:                  uuid.New(),
			InventoryID:         current.ID,
			BranchID:            branchID,
			GoodID:              req.GoodID,
			Type:                req.Type,
			Quantity:            recorded,
			QuantityBefore:      current.Quantity,
			QuantityAfter:       newQty,
			UnitCost:            &costSnapshot,
			DestinationBranchID: req.DestinationBranchID,
			Reason:              req.Reason,
			ReferenceNo:         &ref,
			ReasonCode:          req.ReasonCode,
			AdjustmentKind:      req.AdjustmentKind,
			Counterparty:        req.Counterparty,
			CreatedBy:           actor.UserID,
		}
		if err := tx.InsertMovement(ctx, &m); err != nil {
			return err
		}

		updated, movement = inv, m
		return nil
	})
	if err != nil {
		return CreateMovementResponse{}, err
	}

	invs := []InventoryResponse{ToInventoryResponse(updated)}
	s.enrichInventories(ctx, invs)
	movs := []MovementResponse{ToMovementResponse(movement)}
	s.enrichMovements(ctx, movs)

	return CreateMovementResponse{Inventory: invs[0], Movement: movs[0]}, nil
}

// applyMovement returns the resulting balance and the value stored in the
// ledger's "quantity" column (for ADJUSTMENT that is the resulting balance,
// which is what chk_mov_math expects).
func applyMovement(current decimal.Decimal, req CreateMovementRequest) (newQty, recorded decimal.Decimal, err error) {
	switch req.Type {
	case TypeIn:
		qty := *req.Quantity
		next := current.Add(qty)
		if next.GreaterThan(maxAmount) {
			return decimal.Zero, decimal.Zero, ErrQuantityTooLarge
		}
		return next, qty, nil

	case TypeOut:
		qty := *req.Quantity
		if qty.GreaterThan(current) {
			return decimal.Zero, decimal.Zero, &InsufficientStockError{Available: current, Requested: qty}
		}
		return current.Sub(qty), qty, nil

	case TypeAdjustment:
		if req.Delta != nil {
			next := current.Add(*req.Delta)
			if next.IsNegative() {
				return decimal.Zero, decimal.Zero, ErrNegativeBalance
			}
			if next.GreaterThan(maxAmount) {
				return decimal.Zero, decimal.Zero, ErrQuantityTooLarge
			}
			return next, next, nil
		}
		return *req.Quantity, *req.Quantity, nil // absolute resulting quantity

	default:
		return decimal.Zero, decimal.Zero, ErrInvalidMovementType
	}
}

// checkAdjustmentKind makes sure SHORTAGE really lowers and SURPLUS really raises the balance.
func checkAdjustmentKind(kind AdjustmentKind, delta decimal.Decimal) error {
	switch kind {
	case AdjShortage:
		if !delta.IsNegative() {
			return ErrAdjustmentKindMismatch
		}
	case AdjSurplus:
		if !delta.IsPositive() {
			return ErrAdjustmentKindMismatch
		}
	}
	return nil
}

// formatReference builds REC-1001 / OUT-1002 / TRF-1003 / ADJ-2026-1004.
func formatReference(t MovementType, isTransfer bool, now time.Time, seq int64) string {
	switch {
	case t == TypeIn:
		return fmt.Sprintf("REC-%d", seq)
	case t == TypeOut && isTransfer:
		return fmt.Sprintf("TRF-%d", seq)
	case t == TypeOut:
		return fmt.Sprintf("OUT-%d", seq)
	default:
		return fmt.Sprintf("ADJ-%d-%d", now.UTC().Year(), seq)
	}
}

// UpdateStockSettings changes safety level / reorder level / capacity / location of one good.
func (s *Service) UpdateStockSettings(ctx context.Context, actor Actor, requestedBranch *uuid.UUID, goodID uuid.UUID, req StockSettingsRequest) (InventoryResponse, error) {
	branchID, err := s.authorizeRead(actor, requestedBranch)
	if err != nil {
		return InventoryResponse{}, err
	}
	if goodID == uuid.Nil {
		return InventoryResponse{}, ErrGoodRequired
	}

	req.Normalize()
	if err := validateSettingsRequest(req); err != nil {
		return InventoryResponse{}, err
	}

	var updated Inventory
	err = s.store.WithTx(ctx, func(tx TxStore) error {
		current, err := tx.LockOrCreateInventory(ctx, branchID, goodID)
		if err != nil {
			return err
		}

		merged := StockSettings{
			MinStock:     current.MinStock,
			ReorderLevel: current.ReorderLevel,
			MaxStock:     current.MaxStock,
			LocationCode: current.LocationCode,
		}
		if req.MinStock != nil {
			merged.MinStock = *req.MinStock
		}
		if req.ReorderLevel != nil {
			merged.ReorderLevel = *req.ReorderLevel
		}
		if req.MaxStock != nil {
			if req.MaxStock.IsZero() {
				merged.MaxStock = nil
			} else {
				v := *req.MaxStock
				merged.MaxStock = &v
			}
		}
		if req.LocationCode != nil {
			if *req.LocationCode == "" {
				merged.LocationCode = nil
			} else {
				v := *req.LocationCode
				merged.LocationCode = &v
			}
		}
		if merged.MaxStock != nil && merged.MaxStock.LessThan(decimal.Max(merged.MinStock, merged.ReorderLevel)) {
			return ErrInvalidStockSettings
		}

		inv, err := tx.UpdateSettings(ctx, current.ID, merged)
		if err != nil {
			return err
		}
		updated = inv
		return nil
	})
	if err != nil {
		return InventoryResponse{}, err
	}

	out := []InventoryResponse{ToInventoryResponse(updated)}
	s.enrichInventories(ctx, out)
	return out[0], nil
}

// ==================== Reads ====================

// ListInventory — current quantity of each tracked good, with search / category / status filters.
func (s *Service) ListInventory(ctx context.Context, actor Actor, requestedBranch *uuid.UUID, q InventoryQuery) (PageResponse[InventoryResponse], error) {
	branchID, err := s.authorizeRead(actor, requestedBranch)
	if err != nil {
		return PageResponse[InventoryResponse]{}, err
	}
	if q.Status != nil && !q.Status.IsValid() {
		return PageResponse[InventoryResponse]{}, ErrInvalidStatus
	}

	page, limit := NormalizePaging(q.Page, q.Limit)
	filter := InventoryFilter{BranchID: branchID, Status: q.Status, Page: page, Limit: limit}

	// Name / SKU / barcode / category live in the catalog, so resolve them to good IDs first.
	if q.Search != "" || q.Category != "" {
		ids, err := s.catalog.SearchGoods(ctx, GoodSearch{Query: q.Search, Category: q.Category})
		if err != nil {
			return PageResponse[InventoryResponse]{}, fmt.Errorf("catalog search: %w", err)
		}
		filter.FilterByGoods = true
		filter.GoodIDs = ids
	}

	items, total, err := s.store.ListInventories(ctx, filter)
	if err != nil {
		return PageResponse[InventoryResponse]{}, err
	}

	data := make([]InventoryResponse, 0, len(items))
	for _, inv := range items {
		data = append(data, ToInventoryResponse(inv))
	}
	s.enrichInventories(ctx, data)

	return PageResponse[InventoryResponse]{
		Data:       data,
		Pagination: Pagination{Page: page, Limit: limit, Total: total},
	}, nil
}

// GetInventorySummary feeds the list screen: totals, status tabs, category chips, book value.
func (s *Service) GetInventorySummary(ctx context.Context, actor Actor, requestedBranch *uuid.UUID) (InventorySummaryResponse, error) {
	branchID, err := s.authorizeRead(actor, requestedBranch)
	if err != nil {
		return InventorySummaryResponse{}, err
	}

	items, err := s.store.ListAllInventories(ctx, branchID)
	if err != nil {
		return InventorySummaryResponse{}, err
	}
	goods := s.lookupGoods(ctx, goodIDsOf(items))

	var (
		statuses StatusCounts
		cats     = map[string]int{}
		value    = decimal.Zero
		latest   time.Time
	)
	for _, inv := range items {
		switch inv.Status() {
		case StatusCritical:
			statuses.Critical++
		case StatusLow:
			statuses.Low++
		default:
			statuses.Available++
		}
		cats[orDefault(goods[inv.GoodID].Category, uncategorized)]++
		value = value.Add(inv.TotalValue().Round(moneyScale))
		if inv.UpdatedAt.After(latest) {
			latest = inv.UpdatedAt
		}
	}

	categories := make([]CategoryCount, 0, len(cats))
	for name, n := range cats {
		categories = append(categories, CategoryCount{Category: name, Count: n})
	}
	sort.Slice(categories, func(i, j int) bool {
		if categories[i].Count != categories[j].Count {
			return categories[i].Count > categories[j].Count
		}
		return categories[i].Category < categories[j].Category
	})

	var lastUpdated *time.Time
	if !latest.IsZero() {
		lastUpdated = &latest
	}

	return InventorySummaryResponse{
		BranchID:        branchID,
		TotalItems:      len(items),
		CategoriesCount: len(categories),
		Statuses:        statuses,
		Categories:      categories,
		BookValue:       value.StringFixed(moneyScale),
		LastUpdatedAt:   lastUpdated,
		GeneratedAt:     s.now().UTC(),
	}, nil
}

// GetInventoryItem — one good's balance with limits, consumption, last receipt and latest movements.
func (s *Service) GetInventoryItem(ctx context.Context, actor Actor, requestedBranch *uuid.UUID, goodID uuid.UUID) (InventoryItemResponse, error) {
	branchID, err := s.authorizeRead(actor, requestedBranch)
	if err != nil {
		return InventoryItemResponse{}, err
	}
	if goodID == uuid.Nil {
		return InventoryItemResponse{}, ErrGoodRequired
	}

	inv, err := s.store.GetInventory(ctx, branchID, goodID)
	if err != nil {
		return InventoryItemResponse{}, err
	}

	movs, _, err := s.store.ListMovements(ctx, MovementFilter{
		BranchID: branchID,
		GoodID:   &goodID,
		Page:     1,
		Limit:    recentMovementsLimit,
	})
	if err != nil {
		return InventoryItemResponse{}, err
	}

	inType := TypeIn
	receipts, _, err := s.store.ListMovements(ctx, MovementFilter{
		BranchID: branchID,
		GoodID:   &goodID,
		Type:     &inType,
		Page:     1,
		Limit:    1,
	})
	if err != nil {
		return InventoryItemResponse{}, err
	}

	now := s.now()
	outflow, err := s.store.OutflowSince(ctx, branchID, goodID, now.AddDate(0, 0, -consumptionWindowDays))
	if err != nil {
		return InventoryItemResponse{}, err
	}

	recent := make([]MovementResponse, 0, len(movs))
	for _, m := range movs {
		recent = append(recent, ToMovementResponse(m))
	}
	s.enrichMovements(ctx, recent)

	resp := InventoryItemResponse{RecentMovements: recent}

	base := []InventoryResponse{ToInventoryResponse(inv)}
	s.enrichInventories(ctx, base)
	resp.InventoryResponse = base[0]

	if len(receipts) > 0 {
		last := []MovementResponse{ToMovementResponse(receipts[0])}
		s.enrichMovements(ctx, last)
		resp.LastReceipt = &last[0]
	}

	// Average daily consumption and how long the stock will last at that rate.
	avg := outflow.Div(decimal.NewFromInt(consumptionWindowDays))
	if avg.IsPositive() {
		avgStr := avg.StringFixed(moneyScale)
		resp.AvgDailyConsumption = &avgStr

		cover := inv.Quantity.Div(avg)
		coverStr := cover.StringFixed(1)
		resp.DaysOfCover = &coverStr
		if cover.LessThanOrEqual(decimal.NewFromInt(maxCoverDays)) {
			seconds := cover.Mul(decimal.NewFromInt(24 * 3600)).IntPart()
			t := now.UTC().Add(time.Duration(seconds) * time.Second)
			resp.EstimatedStockoutAt = &t
		}
	}

	return resp, nil
}

// ListMovements — audit trail, filterable by good, type, date and free-text search.
func (s *Service) ListMovements(ctx context.Context, actor Actor, requestedBranch *uuid.UUID, f MovementFilter) (PageResponse[MovementResponse], error) {
	branchID, err := s.authorizeRead(actor, requestedBranch)
	if err != nil {
		return PageResponse[MovementResponse]{}, err
	}

	if err := validateMovementFilter(f); err != nil {
		return PageResponse[MovementResponse]{}, err
	}

	f.BranchID = branchID
	f.Page, f.Limit = NormalizePaging(f.Page, f.Limit)
	s.resolveMovementSearch(ctx, &f)

	items, total, err := s.store.ListMovements(ctx, f)
	if err != nil {
		return PageResponse[MovementResponse]{}, err
	}

	data := make([]MovementResponse, 0, len(items))
	for _, m := range items {
		data = append(data, ToMovementResponse(m))
	}
	s.enrichMovements(ctx, data)

	return PageResponse[MovementResponse]{
		Data:       data,
		Pagination: Pagination{Page: f.Page, Limit: f.Limit, Total: total},
	}, nil
}

// GetMovementSummary feeds the movements screen cards (counts per type, net quantity and value).
func (s *Service) GetMovementSummary(ctx context.Context, actor Actor, requestedBranch *uuid.UUID, f MovementFilter) (MovementSummaryResponse, error) {
	branchID, err := s.authorizeRead(actor, requestedBranch)
	if err != nil {
		return MovementSummaryResponse{}, err
	}
	if err := validateMovementFilter(f); err != nil {
		return MovementSummaryResponse{}, err
	}

	f.BranchID = branchID
	s.resolveMovementSearch(ctx, &f)

	rows, err := s.store.MovementSummary(ctx, f)
	if err != nil {
		return MovementSummaryResponse{}, err
	}

	toSummary := func(r TypeAggregate) MovementTypeSummary {
		return MovementTypeSummary{
			Count:       r.Count,
			NetQuantity: r.Quantity.StringFixed(moneyScale),
			NetValue:    r.Value.StringFixed(moneyScale),
		}
	}
	empty := MovementTypeSummary{NetQuantity: decimal.Zero.StringFixed(moneyScale), NetValue: decimal.Zero.StringFixed(moneyScale)}
	resp := MovementSummaryResponse{BranchID: branchID, In: empty, Out: empty, Adjustment: empty, From: f.From, To: f.To}

	for _, r := range rows {
		resp.TotalCount += r.Count
		switch MovementType(r.Type) {
		case TypeIn:
			resp.In = toSummary(r)
		case TypeOut:
			resp.Out = toSummary(r)
		case TypeAdjustment:
			resp.Adjustment = toSummary(r)
		}
	}
	return resp, nil
}

// resolveMovementSearch turns the free-text search into good IDs through the catalog.
// If the catalog is unavailable the search still works on the reference number.
func (s *Service) resolveMovementSearch(ctx context.Context, f *MovementFilter) {
	if f.Search == "" {
		return
	}
	ids, err := s.catalog.SearchGoods(ctx, GoodSearch{Query: f.Search})
	if err != nil {
		s.log.WarnContext(ctx, "catalog search failed, searching reference numbers only", "err", err)
		return
	}
	f.SearchGoodIDs = ids
}

func validateMovementFilter(f MovementFilter) error {
	if f.Type != nil && !f.Type.IsValid() {
		return ErrInvalidMovementType
	}
	if f.From != nil && f.To != nil && !f.From.Before(*f.To) {
		return ErrInvalidDateRange
	}
	return nil
}

// GetInventoryValue — value = quantity × unit cost, per good, per category and total.
// top > 0 limits the returned items to the N most valuable (totals still cover everything).
func (s *Service) GetInventoryValue(ctx context.Context, actor Actor, requestedBranch *uuid.UUID, top int) (InventoryValueResponse, error) {
	branchID, err := s.authorizeRead(actor, requestedBranch)
	if err != nil {
		return InventoryValueResponse{}, err
	}

	items, err := s.store.ListAllInventories(ctx, branchID)
	if err != nil {
		return InventoryValueResponse{}, err
	}
	goods := s.lookupGoods(ctx, goodIDsOf(items))

	type agg struct {
		count int
		qty   decimal.Decimal
		value decimal.Decimal
	}
	var (
		totalQty   = decimal.Zero
		totalValue = decimal.Zero
		cats       = map[string]*agg{}
		units      = map[string]decimal.Decimal{}
		out        = make([]InventoryValueItem, 0, len(items))
	)

	for _, inv := range items {
		// Round each line first so the total equals the sum of displayed lines.
		value := inv.TotalValue().Round(moneyScale)
		totalQty = totalQty.Add(inv.Quantity)
		totalValue = totalValue.Add(value)

		info := goods[inv.GoodID]
		category := orDefault(info.Category, uncategorized)
		unit := orDefault(info.Unit, unknownUnit)

		a := cats[category]
		if a == nil {
			a = &agg{qty: decimal.Zero, value: decimal.Zero}
			cats[category] = a
		}
		a.count++
		a.qty = a.qty.Add(inv.Quantity)
		a.value = a.value.Add(value)

		units[unit] = units[unit].Add(inv.Quantity)

		out = append(out, InventoryValueItem{
			InventoryID: inv.ID,
			GoodID:      inv.GoodID,
			Good:        goodSummary(goods, inv.GoodID),
			Status:      inv.Status(),
			Quantity:    inv.Quantity.StringFixed(moneyScale),
			UnitCost:    inv.UnitCost.StringFixed(moneyScale),
			Value:       value.StringFixed(moneyScale),
		})
	}

	byCategory := make([]CategoryValue, 0, len(cats))
	hundred := decimal.NewFromInt(100)
	for name, a := range cats {
		share := decimal.Zero
		if totalValue.IsPositive() {
			share = a.value.Div(totalValue).Mul(hundred)
		}
		byCategory = append(byCategory, CategoryValue{
			Category:   name,
			ItemsCount: a.count,
			Quantity:   a.qty.StringFixed(moneyScale),
			Value:      a.value.StringFixed(moneyScale),
			SharePct:   share.StringFixed(1),
		})
	}
	sort.Slice(byCategory, func(i, j int) bool {
		vi, _ := decimal.NewFromString(byCategory[i].Value)
		vj, _ := decimal.NewFromString(byCategory[j].Value)
		if !vi.Equal(vj) {
			return vi.GreaterThan(vj)
		}
		return byCategory[i].Category < byCategory[j].Category
	})

	byUnit := make([]UnitQuantity, 0, len(units))
	for unit, qty := range units {
		byUnit = append(byUnit, UnitQuantity{Unit: unit, Quantity: qty.StringFixed(moneyScale)})
	}
	sort.Slice(byUnit, func(i, j int) bool { return byUnit[i].Unit < byUnit[j].Unit })

	if top > 0 && len(out) > top {
		out = out[:top]
	}

	return InventoryValueResponse{
		BranchID:       branchID,
		ItemsCount:     len(items),
		TotalQuantity:  totalQty.StringFixed(moneyScale),
		QuantityByUnit: byUnit,
		TotalValue:     totalValue.StringFixed(moneyScale),
		ByCategory:     byCategory,
		Items:          out,
		ValuedAt:       s.now().UTC(),
	}, nil
}

// ==================== Authorization & validation ====================

func (s *Service) authorizeRead(actor Actor, requested *uuid.UUID) (uuid.UUID, error) {
	if err := validateActor(actor); err != nil {
		return uuid.Nil, err
	}
	return resolveBranch(actor, requested)
}

func validateActor(a Actor) error {
	if a.UserID == uuid.Nil || !a.Role.IsValid() {
		return ErrMissingIdentity
	}
	return nil
}

// resolveBranch decides which branch the request acts on.
//   - CEO: the requested branch, or their header branch if none was given.
//   - BRANCH_MANAGER: always their own branch; requesting another one is forbidden.
func resolveBranch(a Actor, requested *uuid.UUID) (uuid.UUID, error) {
	if requested != nil && *requested == uuid.Nil {
		requested = nil
	}

	if a.Role.CanAccessAllBranches() {
		if requested != nil {
			return *requested, nil
		}
		if a.BranchID != uuid.Nil {
			return a.BranchID, nil
		}
		return uuid.Nil, ErrInvalidBranch
	}

	if a.BranchID == uuid.Nil {
		return uuid.Nil, ErrMissingIdentity
	}
	if requested != nil && *requested != a.BranchID {
		return uuid.Nil, ErrUnauthorizedBranch
	}
	return a.BranchID, nil
}

func validateMovementRequest(req CreateMovementRequest) error {
	if req.GoodID == uuid.Nil {
		return ErrGoodRequired
	}
	if !req.Type.IsValid() {
		return ErrInvalidMovementType
	}

	switch req.Type {
	case TypeIn, TypeOut:
		if req.Delta != nil {
			return ErrDeltaNotAllowed
		}
		if req.Quantity == nil {
			return ErrQuantityRequired
		}
		if !req.Quantity.IsPositive() {
			return ErrInvalidQuantity
		}
		if err := checkAmount(*req.Quantity); err != nil {
			return err
		}

	case TypeAdjustment:
		switch {
		case req.Quantity != nil && req.Delta != nil:
			return ErrQuantityAndDelta
		case req.Quantity == nil && req.Delta == nil:
			return ErrQuantityRequired
		case req.Quantity != nil:
			if req.Quantity.IsNegative() {
				return ErrNegativeQuantity
			}
			if err := checkAmount(*req.Quantity); err != nil {
				return err
			}
		default: // delta
			if req.Delta.IsZero() {
				return ErrZeroDelta
			}
			if err := checkAmount(req.Delta.Abs()); err != nil {
				return err
			}
		}
	}

	if req.ExpectedQuantityBefore != nil {
		if req.ExpectedQuantityBefore.IsNegative() {
			return ErrNegativeQuantity
		}
		if err := checkAmount(*req.ExpectedQuantityBefore); err != nil {
			return err
		}
	}

	if req.UnitCost != nil {
		if req.Type != TypeIn {
			return ErrUnitCostNotAllowed
		}
		if req.UnitCost.IsNegative() {
			return ErrInvalidUnitCost
		}
		if err := checkAmount(*req.UnitCost); err != nil {
			return err
		}
	}

	if req.Type.RequiresReason() && req.Reason == nil {
		return ErrReasonRequired
	}

	if req.ReasonCode != nil {
		if !req.ReasonCode.IsValid() {
			return ErrInvalidReasonCode
		}
		if !req.Type.RequiresReason() {
			return ErrReasonCodeNotAllowed
		}
	}

	if req.AdjustmentKind != nil {
		if !req.AdjustmentKind.IsValid() {
			return ErrInvalidAdjustmentKind
		}
		if req.Type != TypeAdjustment {
			return ErrAdjustmentKindNotAllowed
		}
	}

	if req.Counterparty != nil && utf8.RuneCountInString(*req.Counterparty) > maxTextLen {
		return ErrTextTooLong
	}

	if req.DestinationBranchID != nil {
		if req.Type != TypeOut || *req.DestinationBranchID == uuid.Nil {
			return ErrInvalidDestination
		}
	}
	return nil
}

func validateSettingsRequest(req StockSettingsRequest) error {
	if req.MinStock == nil && req.ReorderLevel == nil && req.MaxStock == nil && req.LocationCode == nil {
		return ErrNoSettingsProvided
	}
	for _, d := range []*decimal.Decimal{req.MinStock, req.ReorderLevel, req.MaxStock} {
		if d == nil {
			continue
		}
		if d.IsNegative() {
			return ErrNegativeSetting
		}
		if err := checkAmount(*d); err != nil {
			return err
		}
	}
	if req.LocationCode != nil && utf8.RuneCountInString(*req.LocationCode) > maxLocationLen {
		return ErrTextTooLong
	}
	return nil
}

// checkAmount enforces NUMERIC(12,2): max 2 decimals and within range.
func checkAmount(d decimal.Decimal) error {
	if !d.Equal(d.Round(moneyScale)) {
		return ErrInvalidPrecision
	}
	if d.GreaterThan(maxAmount) {
		return ErrQuantityTooLarge
	}
	return nil
}