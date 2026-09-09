-- A set of map sheets: most often "Patruljer" and one for everybody else (PRD 010).
--
-- # Why this is a table and not a string on kort
--
-- An earlier draft had `setName` as free text on each sheet, with sets derived by grouping. That
-- does not survive teamType: the team type is a property of the set as a whole, so storing it per
-- sheet means five sheets in one set each carry a copy that can disagree, and "which set is the
-- spejder set?" becomes a question with five possibly-conflicting answers. It also let
-- "Patruljer" and "patruljer" drift into two sets.
--
-- Sets stay fully dynamic all the same: the operator creates them, so a year with three sets
-- needs no code change. What this buys is that each one exists exactly once.
CREATE TABLE IF NOT EXISTS kortsaet (
    id VARCHAR(99) NOT NULL,
    year VARCHAR(99) NOT NULL,
    version INT NOT NULL DEFAULT 0,

    name VARCHAR(199) NOT NULL DEFAULT "",

    -- Which team type this set is *specifically for* — not "who may use it".
    --
    -- NULL is the ordinary case, not a missing value: the crew set is for gøglere, banditter and
    -- crew, who are not one team type, and klaner normally draw from it too. Forcing a value
    -- would mean inventing a fictional one.
    --
    -- Deliberately **not unique**. Several sets may carry the same team type: it is a filter that
    -- yields the candidate sheets for a team type, which is exactly what QR linking needs, not a
    -- key. A uniqueness constraint would buy a property nobody consumes and would block a year
    -- that splits its patrol maps into two sets (PRD 010 §8).
    teamType VARCHAR(20) NULL DEFAULT NULL,

    sortOrder INT NOT NULL DEFAULT 0,

    PRIMARY KEY (id),
    KEY year_sort (year, sortOrder)
);
