CREATE VIEW masked."api_keys" AS
SELECT
    "id"
FROM public."api_keys";
COMMENT ON COLUMN masked."api_keys"."id" IS 'pii:allow';
GRANT SELECT ON masked."api_keys" TO analyst;

CREATE VIEW masked."contacts" AS
SELECT
    "id",
    "org_id",
    lower(substring("email" FROM '^[^@[:space:],;<>()/:?=|\\]+@([[:alnum:]](?:[[:alnum:]-]*[[:alnum:]])?(?:\.[[:alnum:]](?:[[:alnum:]-]*[[:alnum:]])?)+)$')) AS "email_domain",
    'p_' || left(encode(public.hmac(convert_to("org_id"::text || '|' || lower("email"::text), 'UTF8'), (SELECT key FROM private.pseudonym_key), 'sha256'), 'hex'), 16) AS "email_pseudonym",
    lower(substring("website" FROM '^https?://(?:[^/?#@[:space:]\\]*@)?([[:alnum:]](?:[[:alnum:]-]*[[:alnum:]])?(?:\.[[:alnum:]](?:[[:alnum:]-]*[[:alnum:]])?)+|[[](?:[0-9a-fA-F]{1,4}(?::[0-9a-fA-F]{1,4}){7}|(?:[0-9a-fA-F]{1,4}(?::[0-9a-fA-F]{1,4}){0,6})?::(?:[0-9a-fA-F]{1,4}(?::[0-9a-fA-F]{1,4}){0,6})?)[]])(?::[0-9]+)?(?:[/?#].*)?$')) AS "website_host"
FROM public."contacts";
COMMENT ON COLUMN masked."contacts"."id" IS 'pii:allow';
COMMENT ON COLUMN masked."contacts"."org_id" IS 'pii:allow';
COMMENT ON COLUMN masked."contacts"."email_domain" IS 'pii:generalise=domain';
COMMENT ON COLUMN masked."contacts"."email_pseudonym" IS 'pii:pseudonymise';
COMMENT ON COLUMN masked."contacts"."website_host" IS 'pii:generalise=host';
GRANT SELECT ON masked."contacts" TO analyst;

CREATE OR REPLACE VIEW masked.classification AS
SELECT * FROM (VALUES
    ('api_keys', 'id', 'allow', NULL, 'id'),
    ('api_keys', 'secret', 'deny=credential', 'credential', NULL),
    ('contacts', 'id', 'allow', NULL, 'id'),
    ('contacts', 'org_id', 'allow', NULL, 'org_id'),
    ('contacts', 'name', 'deny=person', 'person', NULL),
    ('contacts', 'email', 'generalise=domain,pseudonymise', 'person', 'email_domain,email_pseudonym'),
    ('contacts', 'website', 'generalise=host', 'person', 'website_host'),
    ('contacts', 'last_ip', 'deny=person', 'person', NULL)
) AS classification(table_name, column_name, class, reason, visible_as);
GRANT SELECT ON masked.classification TO analyst;
