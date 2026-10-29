-- The number of steps is chosen by the player who invites, not by the one who joins.
-- Invites created before have 5 steps, the default of the interface.
ALTER TABLE "invites" ADD COLUMN "steps_amount" INTEGER NOT NULL DEFAULT 5 CHECK ("steps_amount" > 0);
ALTER TABLE "invites" ALTER COLUMN "steps_amount" DROP DEFAULT;
