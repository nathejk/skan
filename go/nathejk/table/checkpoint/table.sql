CREATE TABLE IF NOT EXISTS checkpoint (
    id VARCHAR(99) NOT NULL,
    year VARCHAR(99) NOT NULL,
    version INT NOT NULL DEFAULT 0,
    checkgroupId VARCHAR(99) NOT NULL,
    name VARCHAR(99) NOT NULL DEFAULT "",
    sortOrder TINYINT,
    openFromUts INT NOT NULL DEFAULT 0,
    openUntilUts INT NOT NULL DEFAULT 0,
    openDuration INT NOT NULL DEFAULT 0,
    latitude FLOAT,
    longitude FLOAT,
    address TEXT NOT NULL DEFAULT "",
    description TEXT NOT NULL DEFAULT "",
    PRIMARY KEY (id)
);
