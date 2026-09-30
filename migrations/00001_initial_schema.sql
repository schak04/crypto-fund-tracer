-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS investigation (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_investigation_created_at ON investigation(created_at);
CREATE INDEX idx_investigation_status ON investigation(status);

CREATE TABLE IF NOT EXISTS address (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    address TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS investigation_address (
    investigation_id UUID NOT NULL REFERENCES investigation(id) ON DELETE CASCADE,
    address_id UUID NOT NULL REFERENCES address(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('suspect', 'intermediary', 'destination', 'other')),
    PRIMARY KEY (investigation_id, address_id)
);

CREATE INDEX idx_investigation_address_address_id ON investigation_address(address_id);

CREATE TABLE IF NOT EXISTS transaction (
    tx_hash TEXT PRIMARY KEY,
    tx_time TIMESTAMPTZ NOT NULL,
    amount NUMERIC NOT NULL,
    raw_reference TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS investigation_transaction (
    investigation_id UUID NOT NULL REFERENCES investigation(id) ON DELETE CASCADE,
    tx_hash TEXT NOT NULL REFERENCES transaction(tx_hash) ON DELETE CASCADE,
    trace_depth INT NOT NULL DEFAULT 1,
    PRIMARY KEY (investigation_id, tx_hash)
);

CREATE INDEX idx_investigation_transaction_tx_hash ON investigation_transaction(tx_hash);

CREATE TABLE IF NOT EXISTS transaction_edge (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tx_hash TEXT NOT NULL REFERENCES transaction(tx_hash) ON DELETE CASCADE,
    source_address_id UUID NOT NULL REFERENCES address(id) ON DELETE CASCADE,
    destination_address_id UUID NOT NULL REFERENCES address(id) ON DELETE CASCADE,
    amount NUMERIC NOT NULL
);

CREATE INDEX idx_transaction_edge_tx_hash ON transaction_edge(tx_hash);
CREATE INDEX idx_transaction_edge_source_address_id ON transaction_edge(source_address_id);
CREATE INDEX idx_transaction_edge_destination_address_id ON transaction_edge(destination_address_id);

CREATE TABLE IF NOT EXISTS vasp (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS vasp_address (
    vasp_id UUID NOT NULL REFERENCES vasp(id) ON DELETE CASCADE,
    address_id UUID NOT NULL REFERENCES address(id) ON DELETE CASCADE,
    attribution_status TEXT NOT NULL CHECK (attribution_status IN ('known', 'probable', 'unverified')),
    PRIMARY KEY (vasp_id, address_id)
);

CREATE INDEX idx_vasp_address_address_id ON vasp_address(address_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS vasp_address;
DROP TABLE IF EXISTS vasp;
DROP TABLE IF EXISTS transaction_edge;
DROP TABLE IF EXISTS investigation_transaction;
DROP TABLE IF EXISTS transaction;
DROP TABLE IF EXISTS investigation_address;
DROP TABLE IF EXISTS address;
DROP TABLE IF EXISTS investigation;

-- +goose StatementEnd
