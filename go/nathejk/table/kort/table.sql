-- The maps we print and hand out during the event (PRD 010).
--
-- A row is one *sheet* — one thing physically handed to a team, carrying one QR code, causing
-- one reveal. That is why a double-sided A3 is a single row with two extents rather than two
-- rows: splitting it to record its geometry would double-count the handover, which is the very
-- thing a consuming app reasons about when it decides what a patrol may see.
--
-- # checkpointIds is an array, on purpose
--
-- The relation is read in exactly one direction — given a map, which checkpoints — and we will
-- never ask which maps contain a given checkpoint (PRD 010 §8). A join table would therefore
-- cost a row per assignment and a join on every read to answer a question nobody asks.
--
-- The price is paid in the cascade: removing a deleted checkpoint from every map is JSON
-- surgery rather than `DELETE WHERE checkpointId = ?` (task 123). At ~15 maps a year that scan
-- is free, and the read model rebuilds from JetStream, so a bug there is fixed by replay rather
-- than by migration.
--
-- TEXT rather than MariaDB's JSON type, following dispatch_task.memberIds: JSON is an alias for
-- LONGTEXT here anyway, the JSON functions work on TEXT, and staying with the established
-- spelling keeps one convention in the codebase.
CREATE TABLE IF NOT EXISTS kort (
    id VARCHAR(99) NOT NULL,
    year VARCHAR(99) NOT NULL,
    version INT NOT NULL DEFAULT 0,

    -- The set this sheet belongs to (kortsaet, task 122). Exactly one: a sheet is printed for
    -- one audience. Not a foreign key — no projection in this codebase declares one, because
    -- replay delivers events in stream order and a map may legitimately be materialised before
    -- its set.
    kortsaetId VARCHAR(99) NOT NULL DEFAULT "",

    name VARCHAR(199) NOT NULL DEFAULT "",

    -- a4 | a3 | skitse | andet. A skitse is the interesting one: a hand-drawn slip with no QR
    -- code and usually no extent, whose only trace in the system is its checkpoint list.
    format VARCHAR(20) NOT NULL DEFAULT "",

    note TEXT NOT NULL DEFAULT "",

    -- Handout order along the route, within a set.
    sortOrder INT NOT NULL DEFAULT 0,

    -- Where this sheet is handed out: the id of the checkgroup whose post gives it to the team,
    -- or "" for "at the QR scan".
    --
    -- This is the sheet's *reveal trigger*, which is why it is worth a column even though PRD 010
    -- originally said handout location would not be recorded. Two things changed the answer: the
    -- consuming app has to decide when a sheet's checkpoints become visible to the scout, and the
    -- planner does know the answer in advance — a sheet is handed out at a specific post, by plan,
    -- and the exceptions are handed over at the scan of the sheet's own QR.
    --
    -- "" is the default and means the QR rule, which is the behaviour that existed before this
    -- column: an unconfigured sheet therefore keeps working exactly as it did.
    --
    -- Not a foreign key (no projection here declares one), and a checkgroup that has since been
    -- deleted is treated as "" on read — see querier.Maps.
    handoutCheckgroupId VARCHAR(99) NOT NULL DEFAULT "",

    -- JSON array of checkpoint ids drawn on this sheet. `[]` and not NULL when empty: a pure
    -- overview map for drivers legitimately has none, and every reader should decode an array
    -- rather than branch on NULL.
    checkpointIds TEXT NOT NULL DEFAULT "[]",

    -- JSON array of 0-2 {northWest:{latitude,longitude}, southEast:{...}} rectangles.
    --
    -- A list because a double-sided sheet shows two different areas. Nothing records which is
    -- the front and which the back, and the checkpoints are not split per side: both sides are
    -- handed over at once, so the distinction has no consumer (PRD 010 §5).
    --
    -- Empty is normal — a skitse has no extent worth recording. The two-item cap is a UI
    -- convention, not a constraint here, so a future three-panel fold needs no schema change.
    extents TEXT NOT NULL DEFAULT "[]",

    PRIMARY KEY (id),
    -- The only read: a year's maps in handout order. Both `GET /api/kort` and the settings
    -- modal want exactly this.
    KEY year_sort (year, kortsaetId, sortOrder)
);
