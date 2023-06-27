CREATE TABLE report_dashboard (
    date_time BIGINT NOT NULL,
    total_opened_app INT DEFAULT 0 NOT NULL,
    -- coupons
    total_transaction_of_redeemed_coupons INT DEFAULT 0 NOT NULL,
    total_redeemed_coupon_addresses INT DEFAULT 0 NOT NULL,
    total_asa_of_redeemed_coupons INT DEFAULT 0 NOT NULL,
    -- staking
    total_staking_transactions INT DEFAULT 0 NOT NULL,
    total_staking_addresses INT DEFAULT 0 NOT NULL,
    total_asa_staked INT DEFAULT 0 NOT NULL,
    -- NFT
    total_nft_transfer_transactions INT DEFAULT 0 NOT NULL,
    total_nft_active_addresses INT DEFAULT 0 NOT NULL,
    total_asa_of_nfts INT DEFAULT 0 NOT NULL,
    -- Astra Rewards app
    total_transactions_from_astra_rewards INT DEFAULT 0 NOT NULL,
    total_active_addreeeses_from_astra_rewards INT DEFAULT 0 NOT NULL,
    total_volume_from_astra_rewards INT DEFAULT 0 NOT NULL,
    total_value_of_redeemed_coupons_in_vnd INT DEFAULT 0 NOT NULL,
    total_new_addresses BIGINT DEFAULT 0 NOT NULL,
    --
    total_asa_withdrawn_from_tiki INT DEFAULT 0 NOT NULL,
    total_asa_on_chain_rewards INT DEFAULT 0 NOT NULL,
);