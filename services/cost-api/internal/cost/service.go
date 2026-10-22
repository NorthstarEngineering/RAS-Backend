package cost

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Service struct {
	db   *sqlx.DB
	repo Repository
}

func NewService(db *sqlx.DB, repo Repository) *Service {
	return &Service{db: db, repo: repo}
}

func (s *Service) SetCost(ctx context.Context, in SetCostRequest) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// 1. حفظ التكلفة الإجمالية
	cost := &MenuItemCost{
		MenuItemID: in.MenuItemID,
		CostAmount: in.CostAmount,
	}
	if err := s.repo.UpsertCost(ctx, cost); err != nil {
		return err
	}

	// 2. مسح المكونات القديمة وإضافة الجديدة (لو موجودة)
	if err := s.repo.DeleteComponentsByMenuItem(ctx, tx, in.MenuItemID); err != nil {
		return err
	}

	for _, compReq := range in.Components {
		comp := &RecipeComponent{
			ID:         uuid.NewString(),
			MenuItemID: in.MenuItemID,
			GoodID:     compReq.GoodID,
			Quantity:   compReq.Quantity,
			Unit:       compReq.Unit,
		}
		if err := s.repo.CreateComponent(ctx, tx, comp); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Service) GetMenuItemCosting(ctx context.Context, menuItemID string) (*MenuItemCostingResponse, error) {
	cost, err := s.repo.GetCostByMenuItemID(ctx, menuItemID)
	if err != nil {
		return nil, err
	}
	if cost == nil {
		return nil, nil // مفيش تكلفة مسجلة
	}

	price, err := s.repo.GetMenuItemPrice(ctx, menuItemID)
	if err != nil {
		return nil, err
	}

	components, err := s.repo.GetComponentsByMenuItem(ctx, menuItemID)
	if err != nil {
		return nil, err
	}

	resp := cost.ToCostingResponse(price, components)
	return &resp, nil
}
