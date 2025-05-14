-- +goose Up
CREATE TABLE foreign_exchange (
    id UUID PRIMARY KEY,
    currency_code VARCHAR(3) NOT NULL,
    buying_rate DECIMAL(10, 4) NOT NULL,
    selling_rate DECIMAL(10, 4) NOT NULL, 
    middle_rate DECIMAL(10, 4) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    date DATE NOT NULL,    
    CONSTRAINT uq_currency_date UNIQUE (currency_code, date)
);


-- +goose Down
DROP TABLE IF EXISTS foreign_exchange;
