CREATE OR REPLACE VIEW `project.masked.events` (
  `id` OPTIONS(description="pii:allow"),
  `contact_domain` OPTIONS(description="pii:generalise=domain"),
  `properties` OPTIONS(description="pii:record 2 of 3 fields exposed; the view description lists each field"),
  `places` OPTIONS(description="pii:record 3 of 4 fields exposed; the view description lists each field")
)
OPTIONS(description="Masked view of the events table. A field not listed as exposed is not available. Classification: id: allow, exposed; secret: deny=credential, not exposed; contact: generalise=domain,pseudonymise, exposed as contact_domain; properties.url: generalise=host, exposed as url_host; properties.email: generalise=domain, exposed as email_domain; properties.ip_addr: deny=person, not exposed; hidden.name: deny=person, not exposed; places.code: allow, exposed; places.note: deny=person, not exposed; places.site: generalise=host, exposed as site_host; places.city.label: allow, exposed.")
AS SELECT
  `id`,
  LOWER(REGEXP_EXTRACT(`contact`, r'^[^@[:space:],;<>()/:?=|\\]+@([[:alnum:]](?:[[:alnum:]-]*[[:alnum:]])?(?:\.[[:alnum:]](?:[[:alnum:]-]*[[:alnum:]])?)+)$')) AS `contact_domain`,
  IF(`properties` IS NULL, NULL, STRUCT(LOWER(REGEXP_EXTRACT(`properties`.`url`, r'^https?://(?:[^/?#@[:space:]\\]*@)?([[:alnum:]](?:[[:alnum:]-]*[[:alnum:]])?(?:\.[[:alnum:]](?:[[:alnum:]-]*[[:alnum:]])?)+|[[](?:[0-9a-fA-F]{1,4}(?::[0-9a-fA-F]{1,4}){7}|(?:[0-9a-fA-F]{1,4}(?::[0-9a-fA-F]{1,4}){0,6})?::(?:[0-9a-fA-F]{1,4}(?::[0-9a-fA-F]{1,4}){0,6})?)[]])(?::[0-9]+)?(?:[/?#].*)?$')) AS `url_host`, LOWER(REGEXP_EXTRACT(`properties`.`email`, r'^[^@[:space:],;<>()/:?=|\\]+@([[:alnum:]](?:[[:alnum:]-]*[[:alnum:]])?(?:\.[[:alnum:]](?:[[:alnum:]-]*[[:alnum:]])?)+)$')) AS `email_domain`)) AS `properties`,
  ARRAY(SELECT AS STRUCT element_0.`code` AS `code`, LOWER(REGEXP_EXTRACT(element_0.`site`, r'^https?://(?:[^/?#@[:space:]\\]*@)?([[:alnum:]](?:[[:alnum:]-]*[[:alnum:]])?(?:\.[[:alnum:]](?:[[:alnum:]-]*[[:alnum:]])?)+|[[](?:[0-9a-fA-F]{1,4}(?::[0-9a-fA-F]{1,4}){7}|(?:[0-9a-fA-F]{1,4}(?::[0-9a-fA-F]{1,4}){0,6})?::(?:[0-9a-fA-F]{1,4}(?::[0-9a-fA-F]{1,4}){0,6})?)[]])(?::[0-9]+)?(?:[/?#].*)?$')) AS `site_host`, IF(element_0.`city` IS NULL, NULL, STRUCT(element_0.`city`.`label` AS `label`)) AS `city` FROM UNNEST(`places`) AS element_0) AS `places`
FROM `project.app.events`