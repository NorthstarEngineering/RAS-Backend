DROP INDEX IF EXISTS idx_mov_branch_good_created;
 
ALTER TABLE inventory_movements
    DROP CONSTRAINT IF EXISTS chk_mov_counterparty_len,
    DROP CONSTRAINT IF EXISTS chk_mov_adjustment_kind,
    DROP CONSTRAINT IF EXISTS chk_mov_reason_code,
    DROP CONSTRAINT IF EXISTS uq_mov_reference_no;
 
ALTER TABLE inventory_movements
    DROP COLUMN IF EXISTS counterparty,
    DROP COLUMN IF EXISTS adjustment_kind,
    DROP COLUMN IF EXISTS reason_code,
    DROP COLUMN IF EXISTS reference_no;
 
DROP SEQUENCE IF EXISTS inventory_reference_seq;
 
ALTER TABLE inventories DROP CONSTRAINT IF EXISTS chk_inventories_limits;
 
ALTER TABLE inventories
    DROP COLUMN IF EXISTS location_code,
    DROP COLUMN IF EXISTS max_stock,
    DROP COLUMN IF EXISTS reorder_level,
    DROP COLUMN IF EXISTS min_stock;
 