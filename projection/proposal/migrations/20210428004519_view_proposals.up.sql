CREATE TABLE view_proposals (
    id BIGSERIAL,
    proposal_id VARCHAR NOT NULL,
    title VARCHAR NOT NULL,
    description VARCHAR NOT NULL,
    type VARCHAR NOT NULL,
    status VARCHAR NOT NULL,
    proposer_address VARCHAR NOT NULL,
    maybe_proposer_operator_address VARCHAR NULL,
    data JSONB NOT NULL,
    initial_deposit JSONB NOT NULL,
    total_deposit JSONB NOT NULL,
    total_vote NUMERIC NOT NULL,
    transaction_hash VARCHAR NOT NULL,
    submit_block_height BIGINT NOT NULL,
    submit_time BIGINT NOT NULL,
    deposit_end_time BIGINT NOT NULL,
    maybe_voting_start_time BIGINT NULL,
    maybe_voting_end_block_height BIGINT NULL,
    maybe_voting_end_time BIGINT NULL,
    PRIMARY KEY (id),
    UNIQUE(proposal_id)
)