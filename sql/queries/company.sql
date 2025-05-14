-- name: UpsertCompany :exec
-- Inserts a new company profile or updates an existing one based on stock_code.
INSERT INTO companies (
    stock_code,
    company_name,
    country_code,
    sector,
    subsector,
    listing_date,            -- Make sure your Go code can pass NULL for this if not available
    profile_source_url,      -- Make sure your Go code can pass NULL
    profile_last_scraped_at, -- This will be set by the query
    created_at,              -- Handled by DB default on INSERT
    updated_at               -- Handled by DB default on INSERT or trigger on UPDATE
) VALUES (
    sqlc.arg(stock_code),
    sqlc.arg(company_name),
    sqlc.arg(country_code),          -- Will be string or NULL from Go
    sqlc.arg(sector),                -- Will be string or NULL from Go
    sqlc.arg(subsector),             -- Will be string or NULL from Go
    sqlc.arg(listing_date),          -- Will be time.Time or NULL from Go
    sqlc.arg(profile_source_url),    -- Will be string or NULL from Go
    NOW(),                           -- Set profile_last_scraped_at to current time
    DEFAULT,                         -- Use default for created_at on new insert
    DEFAULT                          -- Use default for updated_at on new insert
)
ON CONFLICT (stock_code) DO UPDATE SET
    company_name = EXCLUDED.company_name,
    country_code = EXCLUDED.country_code,
    sector = EXCLUDED.sector,
    subsector = EXCLUDED.subsector,
    listing_date = EXCLUDED.listing_date,
    profile_source_url = EXCLUDED.profile_source_url,
    profile_last_scraped_at = NOW(), -- Update this timestamp on conflict
    updated_at = NOW();              -- Explicitly update this via trigger or NOW()

-- name: GetCompanyByStockCode :one
-- Retrieves a company's profile by its stock code.
SELECT * FROM companies
WHERE stock_code = $1;