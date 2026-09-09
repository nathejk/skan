CREATE TABLE IF NOT EXISTS scan (
    id VARCHAR(99),
    year VARCHAR(99),
    qrId VARCHAR(99) NOT NULL,
    teamId VARCHAR(99) NOT NULL,
    teamNumber VARCHAR(99) NOT NULL,
    scannerId VARCHAR(99) NOT NULL,
    scannerPhone VARCHAR(99) NOT NULL,
    uts INT NOT NULL DEFAULT 0,
    latitude VARCHAR(99) NOT NULL,
    longitude VARCHAR(99) NOT NULL,
    KEY year_teamId (year, teamId, uts),
    KEY year_scannerId (year, scannerId, uts),
    -- scannerId is part of the key, not just qrId and uts.
    --
    -- With PRIMARY KEY (qrId, uts) and INSERT IGNORE, any two scans of one code in
    -- the same second silently became one. That is wrong for two different scanners
    -- catching the same patrol at once, which is ordinary play at a checkpoint.
    --
    -- What it still collapses is the *same* scanner recording the same code twice
    -- within one second, which is a double-submit rather than two catches. That is
    -- deliberate, and it is a different mechanism from the 30-minute confirmation
    -- guard: this one silently drops a duplicate, so it must stay narrow enough that
    -- no human could mean it. A confirmed rescan always takes longer than a second,
    -- because a person has to answer a prompt.
    PRIMARY KEY(qrId, uts, scannerId)
);
