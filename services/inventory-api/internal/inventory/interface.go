package inventory
 
import (
	"context"
 
	"github.com/google/uuid"
)
 
// GoodInfo is the catalog data the UI needs next to each balance/movement.
// The inventory module does not own this data; it asks whoever owns goods.
type GoodInfo struct {
	ID       uuid.UUID
	Name     string
	SKU      string
	Category string
	Unit     string // kg, L, pcs ...
	Barcode  string
}
 
// GoodSearch is a catalog search. Query matches name / SKU / barcode / category.
type GoodSearch struct {
	Query    string
	Category string
}
 
// GoodsCatalog is what this module needs from the goods/catalog owner.
type GoodsCatalog interface {
	GetGoods(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]GoodInfo, error)
	SearchGoods(ctx context.Context, q GoodSearch) ([]uuid.UUID, error)
}
 
// UserDirectory resolves user IDs (created_by) to display names.
type UserDirectory interface {
	GetUserNames(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
}
 
// NoopCatalog is a placeholder until the real catalog is wired in main.go.
// Responses simply come back without good details, and text search finds nothing.
type NoopCatalog struct{}
 
func (NoopCatalog) GetGoods(context.Context, []uuid.UUID) (map[uuid.UUID]GoodInfo, error) {
	return map[uuid.UUID]GoodInfo{}, nil
}
 
func (NoopCatalog) SearchGoods(context.Context, GoodSearch) ([]uuid.UUID, error) {
	return []uuid.UUID{}, nil
}
 
// NoopUserDirectory is a placeholder until the real user service is wired.
type NoopUserDirectory struct{}
 
func (NoopUserDirectory) GetUserNames(context.Context, []uuid.UUID) (map[uuid.UUID]string, error) {
	return map[uuid.UUID]string{}, nil
}
 