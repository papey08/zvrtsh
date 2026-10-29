-- Players may agree to finish a session after the current step; the vote of
-- player_1 and player_2 is stored until both agree, then steps_amount is cut.
ALTER TABLE "sessions"
    ADD COLUMN "finish_vote_1" BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN "finish_vote_2" BOOLEAN NOT NULL DEFAULT FALSE;
