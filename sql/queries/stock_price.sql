-- name: UpsertStockPrice :exec
INSERT INTO daily_stock_prices (
    stock_code, price_date, closing_price, source_url, extracted_at
) VALUES (
    sqlc.arg(stock_code), sqlc.arg(price_date), sqlc.arg(closing_price), sqlc.arg(source_url), CURRENT_TIMESTAMP
)
ON CONFLICT (stock_code, price_date) DO UPDATE SET
    closing_price = EXCLUDED.closing_price,
    source_url = EXCLUDED.source_url,
    extracted_at = CURRENT_TIMESTAMP;

-- name: GetStockPrice :one
SELECT * FROM daily_stock_prices
WHERE stock_code = sqlc.arg(stock_code) AND price_date = sqlc.arg(price_date) -- Use named args here too
LIMIT 1;

-- name: GetStockPricesWithDetailsByCodeAndDateRange :many
SELECT
    c.company_name,
    dsp.price_date,
    dsp.closing_price,
    dsp.stock_code -- Good to return for frontend mapping/debugging
FROM
    daily_stock_prices dsp
JOIN
    companies c ON dsp.stock_code = c.stock_code
WHERE
    dsp.stock_code = sqlc.arg(stock_code)
    AND dsp.price_date >= sqlc.arg(start_date)
    AND dsp.price_date <= sqlc.arg(end_date)
ORDER BY
    dsp.price_date ASC;