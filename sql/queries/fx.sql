-- name: UpsertForeignExchange :exec
INSERT INTO foreign_exchange (
    id, currency_code, buying_rate, selling_rate, middle_rate, created_at, date
) VALUES (
    -- Name all parameters explicitly
    sqlc.arg(id), sqlc.arg(currency_code), sqlc.arg(buying_rate),
    sqlc.arg(selling_rate), sqlc.arg(middle_rate), sqlc.arg(created_at), sqlc.arg(date)
)
ON CONFLICT (currency_code, date) DO UPDATE SET
    buying_rate = EXCLUDED.buying_rate,
    selling_rate = EXCLUDED.selling_rate,
    middle_rate = EXCLUDED.middle_rate,
    created_at = EXCLUDED.created_at
;

-- name: GetForeignExchangeByCurrencyAndDateRange :many
SELECT
    date,
    middle_rate -- Adjust if you want other rates
FROM foreign_exchange
WHERE
    currency_code = sqlc.arg(currency_code) -- Explicitly name currency_code
    AND date >= sqlc.arg(start_date)        -- Explicitly name start_date
    AND date <= sqlc.arg(end_date)          -- Explicitly name end_date
ORDER BY
    date ASC;