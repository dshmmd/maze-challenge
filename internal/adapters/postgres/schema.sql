-- Dragon Market schema. Applied idempotently on startup.

CREATE TABLE IF NOT EXISTS guilds (
    id        TEXT PRIMARY KEY,
    total     BIGINT NOT NULL DEFAULT 0,
    reserved  BIGINT NOT NULL DEFAULT 0,
    daily_cap BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS items (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    rarity          TEXT NOT NULL,
    seller_guild_id TEXT NOT NULL,
    price           BIGINT NOT NULL DEFAULT 0,
    quantity        INT NOT NULL DEFAULT 0,
    status          TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS auctions (
    id                TEXT PRIMARY KEY,
    item_id           TEXT NOT NULL REFERENCES items(id),
    seller_guild_id   TEXT NOT NULL,
    start_price       BIGINT NOT NULL,
    ends_at           TIMESTAMPTZ NOT NULL,
    extension_seconds BIGINT NOT NULL,
    status            TEXT NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL
);

-- At most one active auction per item (R3), enforced by the database.
CREATE UNIQUE INDEX IF NOT EXISTS one_active_auction_per_item
    ON auctions (item_id) WHERE status = 'active';

CREATE TABLE IF NOT EXISTS bids (
    id         TEXT PRIMARY KEY,
    auction_id TEXT NOT NULL REFERENCES auctions(id),
    guild_id   TEXT NOT NULL,
    amount     BIGINT NOT NULL,
    active     BOOLEAN NOT NULL,
    placed_at  TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS bids_by_auction ON bids (auction_id);

CREATE TABLE IF NOT EXISTS ledger (
    id       TEXT PRIMARY KEY,
    guild_id TEXT NOT NULL,
    type     TEXT NOT NULL,
    amount   BIGINT NOT NULL,
    item_id  TEXT NOT NULL DEFAULT '',
    memo     TEXT NOT NULL DEFAULT '',
    at       TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS ledger_by_guild ON ledger (guild_id, at);
