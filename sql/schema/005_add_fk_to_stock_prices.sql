-- +goose Up
-- This migration adds the foreign key constraint to the 'daily_stock_prices' table,
-- linking its 'stock_code' column to the 'stock_code' primary key in the 'companies' table.
--
-- PRE-REQUISITE: Ensure that every 'stock_code' value currently existing in
-- 'daily_stock_prices' also exists as a primary key in the 'companies' table.
-- If not, this ALTER TABLE command will fail.

ALTER TABLE daily_stock_prices
ADD CONSTRAINT fk_daily_stock_prices_companies -- Explicit name for the FK
FOREIGN KEY (stock_code) REFERENCES companies(stock_code)
ON DELETE RESTRICT  -- Prevent deleting a company if it still has price records.
                    -- Alternatives:
                    -- ON DELETE CASCADE (deletes prices if company is deleted - use with caution)
                    -- ON DELETE SET NULL (sets stock_code in prices to NULL - requires stock_code to be nullable in daily_stock_prices)
ON UPDATE CASCADE;  -- If a stock_code in 'companies' table is updated (rare),
                    -- it will also be updated in 'daily_stock_prices'.

-- +goose Down
-- Reverses the addition of the foreign key constraint
ALTER TABLE daily_stock_prices
DROP CONSTRAINT IF EXISTS fk_daily_stock_prices_companies;