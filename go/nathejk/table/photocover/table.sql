-- Which photograph represents a patrulje.
--
-- Its own table rather than a column on `photo`, for two reasons. The photo
-- projection is a verbatim copy of foto's package (bound for shared-go) and must
-- not diverge here; and `CREATE TABLE IF NOT EXISTS` never alters an existing
-- table, so a new column would silently not appear in any database that already
-- has one. A new table always gets created.
CREATE TABLE IF NOT EXISTS photocover (
    year VARCHAR(99) NOT NULL DEFAULT "",
    teamId VARCHAR(99) NOT NULL DEFAULT "",

    -- The display ref of the chosen photograph, or "" for "no choice" — which is
    -- how a selection is undone. Deleting the row would work equally well for the
    -- read, but a row that says "explicitly none" survives a replay in which the
    -- clearing event arrives before a later selection is reconsidered.
    ref CHAR(64) NOT NULL DEFAULT "",

    selectedAt DATETIME NULL DEFAULT NULL,

    -- One choice per team per year: choosing is idempotent and the newest event
    -- wins, which is exactly what an UPDATE on a single row gives.
    PRIMARY KEY (year, teamId)
);
