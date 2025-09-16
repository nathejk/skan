CREATE TABLE IF NOT EXISTS registrant (
    registrantId VARCHAR(99) NOT NULL,
    email VARCHAR(99) NOT NULL,
    phone VARCHAR(99) NOT NULL,
    pincode VARCHAR(4) NOT NULL,
    PRIMARY KEY (registrantId)
);
