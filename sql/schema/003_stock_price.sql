-- +goose Up
CREATE TABLE daily_stock_prices (
    id SERIAL PRIMARY KEY,              -- Auto-incrementing ID
    stock_code VARCHAR(20) NOT NULL,    -- Stock code (e.g., '1155')
    price_date DATE NOT NULL,           -- The date the price was recorded
    closing_price DECIMAL(12, 4) NOT NULL, -- The closing price (adjust precision/scale as needed)
    source_url VARCHAR(512) NULL,       -- URL where the data was scraped from
    extracted_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL, -- When the data was inserted

    -- Prevent duplicate entries for the same stock on the same day
    UNIQUE (stock_code, price_date)
);

-- Add comments for clarity
COMMENT ON TABLE daily_stock_prices IS 'Stores daily closing stock prices scraped from sources like i3investor.';
COMMENT ON COLUMN daily_stock_prices.stock_code IS 'The stock code/symbol (e.g., from KLSE).';
COMMENT ON COLUMN daily_stock_prices.price_date IS 'The date for which the closing price applies.';
COMMENT ON COLUMN daily_stock_prices.closing_price IS 'The closing stock price.';
COMMENT ON COLUMN daily_stock_prices.source_url IS 'The specific URL the data was scraped from for this entry.';
COMMENT ON COLUMN daily_stock_prices.extracted_at IS 'Timestamp indicating when this row was added or last updated.';

-- Indexes for faster lookups
CREATE INDEX idx_dsp_stock_code ON daily_stock_prices (stock_code);
CREATE INDEX idx_dsp_price_date ON daily_stock_prices (price_date);

-- +goose Down
DROP TABLE IF EXISTS daily_stock_prices;