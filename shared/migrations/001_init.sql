CREATE TABLE accounts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner varchar(255) NOT NULL,
    credit_limit bigint NOT NULL,
    available_credit bigint NOT NULL,
    version bigint NOT NULL DEFAULT 0,
    created_at timestamptz DEFAULT now()
);

CREATE TABLE transactions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id uuid REFERENCES accounts(id),
    idempotency_key varchar(255) UNIQUE,
    tx_type varchar(255) NOT NULL CHECK (tx_type IN ('spend', 'repay')),
    amount bigint,
    status varchar(255) NOT NULL CHECK (status IN ('pending', 'committed', 'rejected')),
    created_at timestamptz DEFAULT now()
);
CREATE INDEX transaction_history_idx ON transactions(account_id, created_at DESC);

CREATE TABLE audit_logs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id uuid REFERENCES accounts(id),
    tx_id uuid REFERENCES transactions(id),
    event_type varchar(255) NOT NULL CHECK (event_type IN ('spend', 'repay', 'limit_breach')),
    amount bigint,
    balance_after bigint,
    recorded_at timestamptz DEFAULT now()
);
CREATE INDEX audit_log_idx ON audit_logs(account_id, recorded_at DESC);