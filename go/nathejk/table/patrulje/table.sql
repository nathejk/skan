CREATE TABLE IF NOT EXISTS patrulje (
    teamId VARCHAR(99) NOT NULL,
    year VARCHAR(99) NOT NULL DEFAULT "",
    teamNumber VARCHAR(99) NOT NULL DEFAULT "",
    name VARCHAR(99) NOT NULL DEFAULT "",
    -- Wide enough for a combined multi-group entry: patrols may be formed from
    -- several scout groups, and the signup carries all of their names in this one
    -- field ("1. Ry gruppe ... + Kgs. Lyngby ... + Solskindstroppen ..."). At
    -- VARCHAR(99) those rows were rejected by MariaDB with error 1406.
    groupName VARCHAR(999) NOT NULL DEFAULT "",
    korps VARCHAR(9) NOT NULL DEFAULT "",
    liga VARCHAR(99) NOT NULL DEFAULT "",
    memberCount INT NOT NULL DEFAULT 0,
    contactName VARCHAR(99) NOT NULL DEFAULT "",
    contactPhone VARCHAR(99) NOT NULL DEFAULT "",
    contactEmail VARCHAR(99) NOT NULL DEFAULT "",
    contactRole VARCHAR(99) NOT NULL DEFAULT "",
    signupStatus VARCHAR(9) NOT NULL DEFAULT "",
    PRIMARY KEY (teamId)
);
