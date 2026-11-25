CREATE OR REPLACE VIEW `acme.masked.events` (
  `id` OPTIONS(description="pii:allow"),
  `properties` OPTIONS(description="pii:record 2 of 2 fields exposed; the view description lists each field")
)
OPTIONS(description="Masked view of events. A field not listed as exposed is not available. Classification: id: allow, exposed; name: deny=person, not exposed; properties.email: generalise=domain, exposed as email_domain; properties.page: allow, exposed.")
AS SELECT
  `id`,
  IF(`properties` IS NULL, NULL, STRUCT(LOWER(REGEXP_EXTRACT(`properties`.`email`, r'^[^@[:space:],;<>()/:?=|\\]+@([[:alnum:]](?:[[:alnum:]-]*[[:alnum:]])?(?:\.[[:alnum:]](?:[[:alnum:]-]*[[:alnum:]])?)+)$')) AS `email_domain`, `properties`.`page` AS `page`)) AS `properties`
FROM `acme.events.events`;

CREATE OR REPLACE VIEW `acme.masked.sessions` (
  `id` OPTIONS(description="pii:allow"),
  `url_host` OPTIONS(description="pii:generalise=host")
)
OPTIONS(description="Masked view of sessions. A field not listed as exposed is not available. Classification: id: allow, exposed; url: generalise=host, exposed as url_host; referrer: deny=person, not exposed.")
AS SELECT
  `id`,
  LOWER(REGEXP_EXTRACT(`url`, r'^https?://(?:[^/?#@[:space:]\\]*@)?([[:alnum:]](?:[[:alnum:]-]*[[:alnum:]])?(?:\.[[:alnum:]](?:[[:alnum:]-]*[[:alnum:]])?)+|[[](?:[0-9a-fA-F]{1,4}(?::[0-9a-fA-F]{1,4}){7}|(?:[0-9a-fA-F]{1,4}(?::[0-9a-fA-F]{1,4}){0,6})?::(?:[0-9a-fA-F]{1,4}(?::[0-9a-fA-F]{1,4}){0,6})?)[]])(?::[0-9]+)?(?:[/?#].*)?$')) AS `url_host`
FROM `acme.events.sessions`;
