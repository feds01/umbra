DROP VIEW IF EXISTS masked."accounts";
CREATE VIEW masked."accounts" AS
SELECT
    "id",
    "org_id",
    lower(substring("website" FROM '^https?://(?:[^/?#@[:space:]\\]*@)?([[:alnum:]](?:[[:alnum:]-]*[[:alnum:]])?(?:\.[[:alnum:]](?:[[:alnum:]-]*[[:alnum:]])?)+|[[](?:[0-9a-fA-F]{1,4}(?::[0-9a-fA-F]{1,4}){7}|(?:[0-9a-fA-F]{1,4}(?::[0-9a-fA-F]{1,4}){0,6})?::(?:[0-9a-fA-F]{1,4}(?::[0-9a-fA-F]{1,4}){0,6})?)[]])(?::[0-9]+)?(?:[/?#].*)?$')) AS "website_host",
    "profile"
FROM public."accounts";
COMMENT ON COLUMN masked."accounts"."id" IS 'pii:allow';
COMMENT ON COLUMN masked."accounts"."org_id" IS 'pii:allow';
COMMENT ON COLUMN masked."accounts"."website_host" IS 'pii:generalise=host';
COMMENT ON COLUMN masked."accounts"."profile" IS 'pii:allow=business_data';
GRANT SELECT ON masked."accounts" TO analyst;

DROP VIEW IF EXISTS masked."api_keys";

DROP VIEW IF EXISTS masked."contacts";
CREATE VIEW masked."contacts" AS
SELECT
    "id",
    "org_id",
    lower(substring("email" FROM '^[^@[:space:],;<>()/:?=|\\]+@([[:alnum:]](?:[[:alnum:]-]*[[:alnum:]])?(?:\.[[:alnum:]](?:[[:alnum:]-]*[[:alnum:]])?)+)$')) AS "email_domain",
    'p_' || left(encode(public.hmac(convert_to("org_id"::text || '|' || lower("email"::text), 'UTF8'), (SELECT key FROM private.pseudonym_key ORDER BY id DESC LIMIT 1), 'sha256'), 'hex'), 16) AS "email_pseudonym",
    'p_' || left(encode(public.hmac(convert_to("org_id"::text || '|' || lower("last_ip"::text), 'UTF8'), (SELECT key FROM private.pseudonym_key ORDER BY id DESC LIMIT 1), 'sha256'), 'hex'), 16) AS "last_ip_pseudonym"
FROM public."contacts";
COMMENT ON COLUMN masked."contacts"."id" IS 'pii:allow';
COMMENT ON COLUMN masked."contacts"."org_id" IS 'pii:allow';
COMMENT ON COLUMN masked."contacts"."email_domain" IS 'pii:generalise=domain';
COMMENT ON COLUMN masked."contacts"."email_pseudonym" IS 'pii:pseudonymise';
COMMENT ON COLUMN masked."contacts"."last_ip_pseudonym" IS 'pii:pseudonymise';
GRANT SELECT ON masked."contacts" TO analyst;
