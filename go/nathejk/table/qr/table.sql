CREATE TABLE IF NOT EXISTS qr (
  -- The event year, taken from the message subject rather than from YEAR, so a
  -- replay of an older year is not relabelled with the current one.
  year VARCHAR(99) NOT NULL DEFAULT "",
  id INT(10) UNSIGNED NOT NULL,
  teamNumber INT(10) unsigned DEFAULT NULL,
  mapCreatedAt datetime DEFAULT NULL,
  mapCreatedBy VARCHAR(99) DEFAULT NULL,
  mapCreatedByPhone VARCHAR(20) COLLATE utf8_unicode_ci DEFAULT NULL,
  -- The kort sheet handed over with this code. "" for codes registered before the map
  -- was recorded, which means "unknown sheet" rather than "no sheet".
  mapId VARCHAR(99) NOT NULL DEFAULT "",
  -- (year, id), not id alone. QR ids are plain integers restarting at 1 for every
  -- event, and the printed stickers may be reused from one year to the next, so an
  -- id is only unique within a year. Keyed on id alone, a reused sticker kept the
  -- previous year's binding -- INSERT IGNORE discarded the new one without error --
  -- and every scan of that map was attributed to a patrol from a past race.
  PRIMARY KEY (`year`, `id`)
)
