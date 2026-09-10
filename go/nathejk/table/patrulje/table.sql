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
    -- How many members are still on the route, maintained by the spejderstatus
    -- projection rather than by this one.
    --
    -- That is deliberate on its part: the mux gives no ordering guarantee between
    -- consumers, so recomputing the count here could read member rows the member
    -- projection had not written yet and land a plausible-looking number that is one out.
    -- The column lives on this table because the count belongs to the team; it is written
    -- next to the member rows it is derived from. See spejderstatus.recomputeActiveMemberCount.
    --
    -- **A started team with zero active members is discontinued.** No event says so and
    -- none needs to: move a member back in and the recompute makes the team active again.
    activeMemberCount INT NOT NULL DEFAULT 0,
    PRIMARY KEY (teamId)
);
