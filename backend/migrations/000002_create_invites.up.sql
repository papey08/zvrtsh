-- One active invite per user: a new invite replaces the previous one.
CREATE TABLE "invites" (
    "code"       VARCHAR(36) PRIMARY KEY,
    "user_name"  VARCHAR(30) NOT NULL UNIQUE REFERENCES "users" ("name") ON DELETE CASCADE,
    "created_at" TIMESTAMPTZ NOT NULL,
    "expires_at" TIMESTAMPTZ NOT NULL
);

CREATE INDEX "invites_expires_at_idx" ON "invites" ("expires_at");
