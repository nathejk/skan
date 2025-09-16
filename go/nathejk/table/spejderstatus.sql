CREATE TABLE IF NOT EXISTS spejderstatus (
    id VARCHAR(99) NOT NULL,
    year VARCHAR(99) NOT NULL,
    status VARCHAR(99) NOT NULL,
    updatedAt VARCHAR(99) NOT NULL,
    PRIMARY KEY (year, id)
);
