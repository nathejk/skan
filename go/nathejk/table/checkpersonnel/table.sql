CREATE TABLE IF NOT EXISTS checkpersonnel (
    id VARCHAR(99) NOT NULL,
    year VARCHAR(99) NOT NULL,
    checkpointId VARCHAR(99) NOT NULL,
    userId VARCHAR(99) NOT NULL,
    startUts INT NOT NULL DEFAULT 0,
    endUts INT NOT NULL DEFAULT 0,
    PRIMARY KEY (id)
);
