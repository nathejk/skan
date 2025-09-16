CREATE TABLE IF NOT EXISTS patruljestatus (
    teamId VARCHAR(99) NOT NULL,
    year VARCHAR(99) NOT NULL,
    startedUts INT NOT NULL DEFAULT 0,
    PRIMARY KEY (teamId)
);
