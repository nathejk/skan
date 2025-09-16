CREATE TABLE IF NOT EXISTS qr (
  id INT(10) UNSIGNED NOT NULL,
  teamNumber INT(10) unsigned DEFAULT NULL,
  mapCreatedAt datetime DEFAULT NULL,
  mapCreatedBy VARCHAR(99) DEFAULT NULL,
  mapCreatedByPhone VARCHAR(20) COLLATE utf8_unicode_ci DEFAULT NULL,
  PRIMARY KEY (`id`)
)
