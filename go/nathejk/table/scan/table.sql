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
    PRIMARY KEY(qrId, uts)
);
