CREATE TABLE "sessions" (
    "id"           UUID        PRIMARY KEY,
    -- player_1 has created the session and owns sheet 0, player_2 owns sheet 1
    "player_1"     VARCHAR(30) NOT NULL REFERENCES "users" ("name") ON DELETE CASCADE,
    "player_2"     VARCHAR(30) NOT NULL REFERENCES "users" ("name") ON DELETE CASCADE,
    "steps_amount" INTEGER     NOT NULL CHECK ("steps_amount" > 0),
    -- grows with every change, clients use it to order snapshots of the session
    "version"      BIGINT      NOT NULL,
    "created_at"   TIMESTAMPTZ NOT NULL,
    "updated_at"   TIMESTAMPTZ NOT NULL,
    CHECK ("player_1" <> "player_2")
);

CREATE INDEX "sessions_player_1_idx" ON "sessions" ("player_1", "updated_at" DESC);
CREATE INDEX "sessions_player_2_idx" ON "sessions" ("player_2", "updated_at" DESC);
CREATE INDEX "sessions_updated_at_idx" ON "sessions" ("updated_at");

-- A row appears when the question of the step is written; answer is NULL until
-- the step is answered. A missing row means the step waits for the question.
CREATE TABLE "steps" (
    "session_id"      UUID     NOT NULL REFERENCES "sessions" ("id") ON DELETE CASCADE,
    "sheet"           SMALLINT NOT NULL CHECK ("sheet" IN (0, 1)),
    "step"            INTEGER  NOT NULL CHECK ("step" >= 0),
    "hidden_question" TEXT     NOT NULL,
    "open_question"   TEXT     NOT NULL,
    "answer"          TEXT,
    PRIMARY KEY ("session_id", "sheet", "step")
);
