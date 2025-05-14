-- +goose Up
-- Create the 'companies' table to store stock profile information
CREATE TABLE companies (
    stock_code VARCHAR(20) NOT NULL PRIMARY KEY, -- Stock code/ticker, primary key
    company_name VARCHAR(255) NOT NULL,          -- Full official name of the company
    country_code VARCHAR(10) NULL,               -- e.g., 'MY' (nullable if not always available)
    sector VARCHAR(255) NULL,                    -- Company's primary sector (nullable)
    subsector VARCHAR(255) NULL,                 -- More specific industry/subsector (nullable)
    listing_date DATE NULL,                      -- Optional: Date the company was listed
    profile_source_url VARCHAR(512) NULL,        -- URL where the profile data was scraped from
    profile_last_scraped_at TIMESTAMP WITH TIME ZONE NULL, -- When the profile was last successfully scraped/updated
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL, -- When the company record was first added
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL  -- When the company record was last updated
);

-- Add comments for clarity
COMMENT ON TABLE companies IS 'Stores profile information for companies listed on stock exchanges.';
COMMENT ON COLUMN companies.stock_code IS 'The unique stock code/ticker symbol (e.g., "1155" for Maybank).';
COMMENT ON COLUMN companies.company_name IS 'The full official name of the company.';
COMMENT ON COLUMN companies.country_code IS 'The country code where the company is primarily listed or operates (e.g., MY).';
COMMENT ON COLUMN companies.sector IS 'The primary economic sector of the company.';
COMMENT ON COLUMN companies.subsector IS 'A more specific subsector or industry classification.';
COMMENT ON COLUMN companies.listing_date IS 'The date the company was listed on the stock exchange.';
COMMENT ON COLUMN companies.profile_source_url IS 'The URL from which the profile information was last scraped.';
COMMENT ON COLUMN companies.profile_last_scraped_at IS 'Timestamp of the last successful profile data scrape for this company.';
COMMENT ON COLUMN companies.created_at IS 'Timestamp when this company record was first created in the database.';
COMMENT ON COLUMN companies.updated_at IS 'Timestamp when this company record was last modified.';

-- Create a trigger function to automatically update the 'updated_at' timestamp
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
   NEW.updated_at = NOW(); -- Set updated_at to the current time
   RETURN NEW;
END;
$$ language 'plpgsql';
-- +goose StatementEnd

-- Create a trigger that calls the function before any UPDATE on the 'companies' table
-- +goose StatementBegin
CREATE TRIGGER trigger_companies_updated_at
BEFORE UPDATE ON companies
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();
-- +goose StatementEnd


-- NOTE: The foreign key constraint addition has been commented out line-by-line.
-- You will add this in a subsequent migration (e.g., 005_add_fk_to_stock_prices.sql)
-- AFTER you have populated the 'companies' table with all stock_codes
-- that exist in 'daily_stock_prices'.

-- -- Add a foreign key constraint to the 'daily_stock_prices' table
-- ALTER TABLE daily_stock_prices
-- ADD CONSTRAINT fk_stock_code_companies
-- FOREIGN KEY (stock_code) REFERENCES companies(stock_code)
-- ON DELETE RESTRICT
-- ON UPDATE CASCADE;


-- Optional: Add an index on company_name if you plan to search by it frequently
CREATE INDEX idx_companies_company_name ON companies (company_name);


-- +goose Down
-- Reverse the operations in reverse order

-- Remove the foreign key constraint from 'daily_stock_prices' first
-- This is kept here because if you ever run down on this migration *after*
-- the FK was added by a later migration, you'd want this part available.
-- However, in the current Up state, it does nothing if the FK isn't present.
ALTER TABLE daily_stock_prices
DROP CONSTRAINT IF EXISTS fk_stock_code_companies;

-- Drop the trigger
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trigger_companies_updated_at ON companies;
-- +goose StatementEnd

-- Drop the trigger function
-- +goose StatementBegin
DROP FUNCTION IF EXISTS update_updated_at_column();
-- +goose StatementEnd

-- Drop the 'companies' table
DROP TABLE IF EXISTS companies;