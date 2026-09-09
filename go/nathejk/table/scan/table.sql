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
    -- How the position was obtained: "gps", "manual", or "" for scans recorded before
    -- this was tracked. A hand-placed marker is only as good as the scanner's sense of
    -- where they stood, so anything reading these positions back needs to be able to
    -- tell them apart -- and "" must not be read as GPS.
    locationSource VARCHAR(9) NOT NULL DEFAULT "",
    -- Radius of confidence in metres as the browser reported it, or "" when unknown
    -- (every hand-placed marker, and every scan predating this column). A GPS fix in a
    -- forest can be hundreds of metres out, so a position without this is a weaker
    -- claim than it appears.
    locationAccuracy VARCHAR(9) NOT NULL DEFAULT "",
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
