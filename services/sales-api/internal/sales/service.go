package sales

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// ============================================================
// Domain Errors
// ============================================================
var (
	ErrShiftNotFound    = errors.New("shift not found")
	ErrShiftNotOpen     = errors.New("shift is not open")
	ErrShiftNotClosed   = errors.New("shift is not closed")
	ErrInvalidChannel   = errors.New("invalid sales channel")
	ErrMenuItemNotFound = errors.New("menu item not found")
	ErrDiscountNotFound = errors.New("discount not found")
)

// ============================================================
// Service
// ============================================================
type Service struct {
	db   *sqlx.DB
	repo Repository
}

func NewService(db *sqlx.DB, repo Repository) *Service {
	return &Service{db: db, repo: repo}
}

// ============================================================
// 1. Open Shift (فتح وردية)
// ============================================================
func (s *Service) OpenShift(ctx context.Context, in OpenShiftRequest) (*Shift, error) {
	now := time.Now().UTC()
	shift := &Shift{
		ID:           uuid.NewString(),
		BranchID:     in.BranchID,
		ManagerID:    in.ManagerID,
		Status:       "OPEN",
		OpenedAt:     now,
		OpeningFloat: in.OpeningFloat,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.repo.CreateShift(ctx, tx, shift); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return shift, nil
}

// ============================================================
// 2. Record Sale (تسجيل بيعة - مطعم أو دليفري)
// ============================================================
func (s *Service) RecordSale(ctx context.Context, in RecordSaleRequest, actorID string) (*Sale, []*SaleItem, error) {
	// 1. التأكد من الوردية
	shift, err := s.repo.GetShiftByID(ctx, in.ShiftID)
	if err != nil {
		return nil, nil, err
	}
	if shift == nil {
		return nil, nil, ErrShiftNotFound
	}
	if shift.Status != "OPEN" {
		return nil, nil, ErrShiftNotOpen // مينفعش نسجل مبيعات في وردية مقفولة
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()
	saleID := uuid.NewString()
	var grossAmount float64 = 0
	var saleItems []*SaleItem

	// 2. معالجة كل صنف (جلب السعر التاريخي وحساب الإجمالي بس، من غير ما نحفظ)
	for _, itemReq := range in.Items {
		// بنجيب السعر من الـ Menu عشان نحفظه تاريخياً (FR-SALES-004)
		unitPrice, err := s.repo.GetMenuItemPrice(ctx, itemReq.MenuItemID)
		if err != nil {
			return nil, nil, ErrMenuItemNotFound
		}

		totalPrice := unitPrice * itemReq.Quantity
		grossAmount += totalPrice

		item := &SaleItem{
			ID:         uuid.NewString(),
			SaleID:     saleID,
			MenuItemID: itemReq.MenuItemID,
			Quantity:   itemReq.Quantity,
			UnitPrice:  unitPrice, // السعر التاريخي
			TotalPrice: totalPrice,
			CreatedAt:  now,
		}

		// شيلنا سطر CreateSaleItem من هنا
		saleItems = append(saleItems, item)
	}

	// 3. حفظ البيعة نفسها الأول (عشان الـ Foreign Key)
	sale := &Sale{
		ID:             saleID,
		ShiftID:        in.ShiftID,
		BranchID:       shift.BranchID,
		Channel:        in.Channel,
		GrossAmount:    grossAmount,
		DiscountAmount: 0, // بيتحسب بعدين لو فيه خصم
		NetAmount:      grossAmount,
		CreatedBy:      actorID,
		CreatedAt:      now,
	}

	if err := s.repo.CreateSale(ctx, tx, sale); err != nil {
		return nil, nil, err
	}

	// 4. دلوقتي نحفظ الأصناف (بعد ما الـ sale اتحفظ)
	for _, item := range saleItems {
		if err := s.repo.CreateSaleItem(ctx, tx, item); err != nil {
			return nil, nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	return sale, saleItems, nil
}

// ============================================================
// 3. Create Discount (إضافة خصم)
// ============================================================
func (s *Service) CreateDiscount(ctx context.Context, in CreateDiscountRequest, actorID string) (*Discount, error) {
	shift, err := s.repo.GetShiftByID(ctx, in.ShiftID)
	if err != nil {
		return nil, err
	}
	if shift == nil {
		return nil, ErrShiftNotFound
	}
	if shift.Status != "OPEN" {
		return nil, ErrShiftNotOpen
	}

	now := time.Now().UTC()
	discount := &Discount{
		ID:        uuid.NewString(),
		ShiftID:   in.ShiftID,
		Amount:    in.Amount,
		Reason:    in.Reason,
		Status:    "ACTIVE",
		CreatedBy: actorID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.repo.CreateDiscount(ctx, tx, discount); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return discount, nil
}

// ============================================================
// 4. Cancel Discount (إلغاء خصم)
// ============================================================
func (s *Service) CancelDiscount(ctx context.Context, discountID string, in CancelDiscountRequest, actorID string) (*Discount, error) {
	d, err := s.repo.GetDiscountByID(ctx, discountID)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, ErrDiscountNotFound
	}
	if d.Status == "CANCELED" {
		return nil, errors.New("discount is already canceled")
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.repo.CancelDiscount(ctx, tx, d.ID, in.Reason, actorID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	d.Status = "CANCELED"
	d.CanceledReason = &in.Reason
	return d, nil
}

// ============================================================
// 5. Close Shift (إغلاق الوردية)
// ============================================================
func (s *Service) CloseShift(ctx context.Context, in CloseShiftRequest) (*Shift, error) {
	shift, err := s.repo.GetShiftByID(ctx, in.ShiftID)
	if err != nil {
		return nil, err
	}
	if shift == nil {
		return nil, ErrShiftNotFound
	}
	if shift.Status != "OPEN" && shift.Status != "REVIEW" {
		return nil, errors.New("shift is already closed")
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// في الحقيقة هنا ممكن نضيف حسابات الخزنة والـ Expected Cash
	// لكن دلوقتي هنقفلها ونغير حالتها بس
	if err := s.repo.UpdateShiftStatus(ctx, tx, shift.ID, "CLOSED"); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	shift.Status = "CLOSED"
	shift.ClosedAt = &now
	return shift, nil
}

// ============================================================
// 6. Get Shift Summary (للـ Reports والـ Shift Closing)
// ============================================================
func (s *Service) GetShiftSummary(ctx context.Context, shiftID string) (*ShiftSummaryResponse, error) {
	// 1. التأكد من وجود الوردية
	shift, err := s.repo.GetShiftByID(ctx, shiftID)
	if err != nil {
		return nil, err
	}
	if shift == nil {
		return nil, ErrShiftNotFound
	}

	// 2. جلب إجماليات القنوات
	channels, err := s.repo.GetShiftChannelSummary(ctx, shiftID)
	if err != nil {
		return nil, err
	}

	// 3. جلب إجماليات الأصناف
	items, err := s.repo.GetShiftItemSummary(ctx, shiftID)
	if err != nil {
		return nil, err
	}

	// 4. حساب الإجماليات العامة
	grossSales, totalDiscounts, err := s.repo.GetShiftSalesTotal(ctx, shiftID)
	if err != nil {
		return nil, err
	}
	netSales := grossSales - totalDiscounts

	// 5. حساب إجمالي عدد الطلبات
	totalOrders := 0
	for _, ch := range channels {
		totalOrders += ch.OrderCount
	}

	return &ShiftSummaryResponse{
		ShiftID:        shiftID,
		Status:         shift.Status,
		GrossSales:     grossSales,
		TotalDiscounts: totalDiscounts,
		NetSales:       netSales,
		TotalOrders:    totalOrders,
		Channels:       channels,
		Items:          items,
	}, nil
}
