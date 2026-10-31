-- Sessions for 2 to 8 players. A session starts in the lobby with an invite
-- code of its own, so the separate invites table is not needed any more.

CREATE TABLE "session_players" (
    "session_id"  UUID        NOT NULL REFERENCES "sessions" ("id") ON DELETE CASCADE,
    -- the order of joining: 0 is the host; the index of the player's sheet
    "idx"         SMALLINT    NOT NULL CHECK ("idx" BETWEEN 0 AND 7),
    "user_name"   VARCHAR(30) NOT NULL REFERENCES "users" ("name") ON DELETE CASCADE,
    -- the offer to finish the session after the current step
    "finish_vote" BOOLEAN     NOT NULL DEFAULT FALSE,
    PRIMARY KEY ("session_id", "idx"),
    UNIQUE ("session_id", "user_name")
);

CREATE INDEX "session_players_user_name_idx" ON "session_players" ("user_name");

INSERT INTO "session_players" ("session_id", "idx", "user_name", "finish_vote")
SELECT "id", 0, "player_1", "finish_vote_1" FROM "sessions"
UNION ALL
SELECT "id", 1, "player_2", "finish_vote_2" FROM "sessions";

ALTER TABLE "sessions"
    DROP COLUMN "player_1",
    DROP COLUMN "player_2",
    DROP COLUMN "finish_vote_1",
    DROP COLUMN "finish_vote_2",
    -- the existing sessions have been started at once
    ADD COLUMN "started"           BOOLEAN     NOT NULL DEFAULT TRUE,
    -- the invite of a session in the lobby
    ADD COLUMN "invite_code"       VARCHAR(36) UNIQUE,
    ADD COLUMN "invite_expires_at" TIMESTAMPTZ,
    ADD CONSTRAINT "sessions_invite_check" CHECK (("invite_code" IS NULL) = ("invite_expires_at" IS NULL));

ALTER TABLE "sessions" ALTER COLUMN "started" DROP DEFAULT;

-- The rows of all steps appear when the session starts, with the players dealt
-- to them: asker writes the question, answerer answers it. NULL texts are not
-- written yet.
ALTER TABLE "steps"
    DROP CONSTRAINT "steps_sheet_check",
    ADD CONSTRAINT "steps_sheet_check" CHECK ("sheet" BETWEEN 0 AND 7),
    ALTER COLUMN "hidden_question" DROP NOT NULL,
    ALTER COLUMN "open_question" DROP NOT NULL,
    ADD COLUMN "asker"    SMALLINT,
    ADD COLUMN "answerer" SMALLINT;

INSERT INTO "steps" ("session_id", "sheet", "step")
SELECT s."id", sheet, step
FROM "sessions" s,
     generate_series(0, 1) AS sheet,
     generate_series(0, s."steps_amount" - 1) AS step
ON CONFLICT DO NOTHING;

-- with two players the owner of a sheet asked on even steps and answered on odd ones
UPDATE "steps"
SET "asker"    = CASE WHEN "step" % 2 = 0 THEN "sheet" ELSE 1 - "sheet" END,
    "answerer" = CASE WHEN "step" % 2 = 0 THEN 1 - "sheet" ELSE "sheet" END;

ALTER TABLE "steps"
    ALTER COLUMN "asker" SET NOT NULL,
    ALTER COLUMN "answerer" SET NOT NULL,
    ADD CONSTRAINT "steps_players_check" CHECK ("asker" <> "answerer"),
    ADD CONSTRAINT "steps_question_check" CHECK (("hidden_question" IS NULL) = ("open_question" IS NULL)),
    ADD CONSTRAINT "steps_answer_check" CHECK ("answer" IS NULL OR "open_question" IS NOT NULL);

DROP TABLE "invites";
