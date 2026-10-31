-- Only started sessions of two players fit the previous schema, the others are lost.
DELETE FROM "sessions"
WHERE NOT "started"
   OR (SELECT count(*) FROM "session_players" p WHERE p."session_id" = "sessions"."id") <> 2;

CREATE TABLE "invites" (
    "code"         VARCHAR(36) PRIMARY KEY,
    "user_name"    VARCHAR(30) NOT NULL UNIQUE REFERENCES "users" ("name") ON DELETE CASCADE,
    "created_at"   TIMESTAMPTZ NOT NULL,
    "expires_at"   TIMESTAMPTZ NOT NULL,
    "steps_amount" INTEGER     NOT NULL CHECK ("steps_amount" > 0)
);

CREATE INDEX "invites_expires_at_idx" ON "invites" ("expires_at");

-- the deal of two players is fixed: the owner of a sheet asks on even steps
DELETE FROM "sessions" s
WHERE EXISTS (
    SELECT 1 FROM "steps" t
    WHERE t."session_id" = s."id"
      AND t."asker" <> CASE WHEN t."step" % 2 = 0 THEN t."sheet" ELSE 1 - t."sheet" END
);

DELETE FROM "steps" WHERE "open_question" IS NULL;

ALTER TABLE "steps"
    DROP CONSTRAINT "steps_players_check",
    DROP CONSTRAINT "steps_question_check",
    DROP CONSTRAINT "steps_answer_check",
    DROP COLUMN "asker",
    DROP COLUMN "answerer",
    DROP CONSTRAINT "steps_sheet_check",
    ADD CONSTRAINT "steps_sheet_check" CHECK ("sheet" IN (0, 1)),
    ALTER COLUMN "hidden_question" SET NOT NULL,
    ALTER COLUMN "open_question" SET NOT NULL;

ALTER TABLE "sessions"
    DROP CONSTRAINT "sessions_invite_check",
    DROP COLUMN "started",
    DROP COLUMN "invite_code",
    DROP COLUMN "invite_expires_at",
    ADD COLUMN "player_1"      VARCHAR(30) REFERENCES "users" ("name") ON DELETE CASCADE,
    ADD COLUMN "player_2"      VARCHAR(30) REFERENCES "users" ("name") ON DELETE CASCADE,
    ADD COLUMN "finish_vote_1" BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN "finish_vote_2" BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE "sessions" s
SET "player_1"      = p1."user_name",
    "finish_vote_1" = p1."finish_vote",
    "player_2"      = p2."user_name",
    "finish_vote_2" = p2."finish_vote"
FROM "session_players" p1, "session_players" p2
WHERE p1."session_id" = s."id" AND p1."idx" = 0
  AND p2."session_id" = s."id" AND p2."idx" = 1;

ALTER TABLE "sessions"
    ALTER COLUMN "player_1" SET NOT NULL,
    ALTER COLUMN "player_2" SET NOT NULL,
    ADD CHECK ("player_1" <> "player_2");

CREATE INDEX "sessions_player_1_idx" ON "sessions" ("player_1", "updated_at" DESC);
CREATE INDEX "sessions_player_2_idx" ON "sessions" ("player_2", "updated_at" DESC);

DROP TABLE "session_players";
