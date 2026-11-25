DROP VIEW IF EXISTS masked."contacts";
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
