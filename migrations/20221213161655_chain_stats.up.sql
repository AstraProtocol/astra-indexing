CREATE TABLE chain_stats (
    date_time BIGINT NOT NULL,
    number_of_transactions INT DEFAULT 0 NOT NULL,
    total_gas_used BIGINT DEFAULT 0 NOT NULL,
    total_fee BIGINT DEFAULT 0 NOT NULL,
);