-- Trading simulation schema (Phase 0 / roadmap Step 4)
-- Applied on first Postgres init via docker-entrypoint-initdb.d

CREATE TABLE stocks (
    symbol          VARCHAR(20) PRIMARY KEY,
    name            VARCHAR(100),
    sector          VARCHAR(50),
    is_active       BOOLEAN DEFAULT TRUE
);

CREATE TABLE ohlcv (
    id              BIGSERIAL PRIMARY KEY,
    symbol          VARCHAR(20) REFERENCES stocks(symbol),
    date            DATE NOT NULL,
    open            NUMERIC(12,4),
    high            NUMERIC(12,4),
    low             NUMERIC(12,4),
    close           NUMERIC(12,4),
    volume          BIGINT,
    UNIQUE(symbol, date)
);
CREATE INDEX idx_ohlcv_symbol_date ON ohlcv(symbol, date);

CREATE TABLE agents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(100),
    model           VARCHAR(50),
    api_key_hash    VARCHAR(255),
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE simulations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(100),
    start_date      DATE,
    end_date        DATE,
    as_of_date      DATE,
    status          VARCHAR(20),
    config          JSONB,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE portfolios (
    id                         BIGSERIAL PRIMARY KEY,
    simulation_id              UUID REFERENCES simulations(id),
    agent_id                   UUID REFERENCES agents(id),
    cash                       NUMERIC(15,4),
    status                     VARCHAR(20) NOT NULL DEFAULT 'active',
    consecutive_missed_ticks   INT NOT NULL DEFAULT 0,
    consecutive_4xx            INT NOT NULL DEFAULT 0,
    UNIQUE(simulation_id, agent_id)
);

CREATE TABLE holdings (
    id              BIGSERIAL PRIMARY KEY,
    portfolio_id  BIGINT REFERENCES portfolios(id),
    symbol          VARCHAR(20),
    quantity        INT,
    avg_buy_price   NUMERIC(12,4),
    UNIQUE(portfolio_id, symbol)
);

CREATE TABLE orders (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    simulation_id   UUID REFERENCES simulations(id),
    agent_id        UUID REFERENCES agents(id),
    symbol          VARCHAR(20),
    order_type      VARCHAR(10),
    side            VARCHAR(4),
    quantity        INT,
    price           NUMERIC(12,4),
    status          VARCHAR(10),
    filled_price    NUMERIC(12,4),
    filled_at       DATE,
    rejection_reason TEXT,
    match_on_date   DATE,
    fees_total      NUMERIC(15,4),
    trade_value     NUMERIC(15,4),
    fee_brokerage   NUMERIC(15,4),
    fee_stt         NUMERIC(15,4),
    fee_gst         NUMERIC(15,4),
    fee_exchange    NUMERIC(15,4),
    fee_sebi        NUMERIC(15,4),
    fee_stamp       NUMERIC(15,4),
    created_at      TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_orders_sim_match_pending ON orders(simulation_id, match_on_date, status)
    WHERE status = 'pending';

CREATE TABLE portfolio_snapshots (
    id              BIGSERIAL PRIMARY KEY,
    simulation_id   UUID REFERENCES simulations(id),
    agent_id        UUID REFERENCES agents(id),
    date            DATE,
    total_value     NUMERIC(15,4),
    cash            NUMERIC(15,4),
    invested_value  NUMERIC(15,4),
    UNIQUE(simulation_id, agent_id, date)
);
