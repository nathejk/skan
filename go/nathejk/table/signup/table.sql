CREATE TABLE IF NOT EXISTS signup (
    teamId VARCHAR(99) NOT NULL,
    teamType VARCHAR(99) NOT NULL,
    name VARCHAR(99) NOT NULL,
    emailPending VARCHAR(99) NOT NULL,
    email VARCHAR(99),
	phonePending VARCHAR(99) NOT NULL,
	phone VARCHAR(99),
	pincode VARCHAR(9),
	createdAt VARCHAR(99),
    PRIMARY KEY (teamId)
);
