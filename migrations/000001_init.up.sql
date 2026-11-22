-- Пользователи. email храним в lower-case (приводим в коде перед сохранением).
CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    email         TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Лоты. Деньги — BIGINT в минимальных единицах (без float).
-- current_price при создании = start_price; leader_id = NULL, пока нет ставок.
CREATE TABLE lots (
    id            BIGSERIAL PRIMARY KEY,
    owner_id      BIGINT      NOT NULL REFERENCES users(id),
    title         TEXT        NOT NULL,
    start_price   BIGINT      NOT NULL CHECK (start_price > 0),
    step          BIGINT      NOT NULL CHECK (step > 0),
    current_price BIGINT      NOT NULL,
    leader_id     BIGINT      REFERENCES users(id),
    status        TEXT        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'finished')),
    ends_at       TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Воркер ищет: WHERE status = 'active' AND ends_at < now();
-- список лотов фильтруется по status.
CREATE INDEX idx_lots_status_ends_at ON lots (status, ends_at);

-- Ставки. История, последние 50 по лоту.
CREATE TABLE bids (
    id         BIGSERIAL PRIMARY KEY,
    lot_id     BIGINT      NOT NULL REFERENCES lots(id) ON DELETE CASCADE,
    user_id    BIGINT      NOT NULL REFERENCES users(id),
    amount     BIGINT      NOT NULL CHECK (amount > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- GET /lots/{id}/bids: WHERE lot_id = $1 ORDER BY created_at DESC LIMIT 50.
CREATE INDEX idx_bids_lot_created ON bids (lot_id, created_at DESC);
