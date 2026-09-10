CREATE TABLE IF NOT EXISTS spejderstatus (
    id VARCHAR(99) NOT NULL,
    year VARCHAR(99) NOT NULL,
    initialTeamId VARCHAR(99) NOT NULL DEFAULT "",
    currentTeamId VARCHAR(99) NOT NULL DEFAULT "",
    status VARCHAR(99) NOT NULL,
    updatedAt DATETIME NOT NULL,
    PRIMARY KEY (year, id),
    KEY year_team_status (year, currentTeamId, status)
);

-- The member's history, one row per lifecycle event.
--
-- spejderstatus holds where a member is *now*, which is what every count and every
-- strength is derived from. This holds how they got there, which is what somebody looking
-- at one member needs: "venter siden 21:40" answers less than "racing at 20:07, waiting at
-- 21:40, in a car at 22:15".
--
-- Keyed by the event's stream sequence **and** the member, so replaying history rebuilds
-- the same rows rather than appending a second copy of every member's past on each
-- restart. The member is part of the key because one event can concern several: a patrol
-- starting puts its whole roster into `racing` from a single message, and a seq-only key
-- would let the first member written silently swallow the rest.
--
-- `status` is the status *after* the event. The previous one is the previous row, so it is
-- not stored twice -- deriving "from" would have meant the projection reading its own
-- current row before writing, which is a read-modify-write in a consumer that has no need
-- of one.
CREATE TABLE IF NOT EXISTS spejderstatuslog (
    seq BIGINT UNSIGNED NOT NULL,
    id VARCHAR(99) NOT NULL,
    year VARCHAR(99) NOT NULL,
    teamId VARCHAR(99) NOT NULL DEFAULT "",
    status VARCHAR(99) NOT NULL,
    event VARCHAR(99) NOT NULL DEFAULT "",
    actorUserId VARCHAR(99) NOT NULL DEFAULT "",
    createdAt DATETIME NOT NULL,
    PRIMARY KEY (seq, id),
    KEY member_seq (year, id, seq)
);
