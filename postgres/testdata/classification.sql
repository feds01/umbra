CREATE OR REPLACE VIEW masked.column_classification AS
SELECT * FROM (VALUES
    ('accounts', 'id', 'allow', NULL, 'id'),
    ('accounts', 'org_id', 'allow', NULL, 'org_id'),
    ('accounts', 'website', 'generalise=host', NULL, 'website_host'),
    ('accounts', 'profile', 'allow=business_data', 'business_data', 'profile'),
    ('accounts', 'notes', 'deny=free_text', 'free_text', NULL),
    ('api_keys', 'secret', 'deny=credential', 'credential', NULL),
    ('contacts', 'id', 'allow', NULL, 'id'),
    ('contacts', 'org_id', 'allow', NULL, 'org_id'),
    ('contacts', 'name', 'deny=person', 'person', NULL),
    ('contacts', 'email', 'generalise=domain,pseudonymise', 'person', 'email_domain,email_pseudonym'),
    ('contacts', 'last_ip', 'pseudonymise', 'person', 'last_ip_pseudonym'),
    ('private.pseudonym_key', 'id', 'allow', NULL, 'id'),
    ('private.pseudonym_key', 'key', 'deny=credential', 'credential', NULL)
) AS classification(table_name, column_name, class, reason, visible_as);
GRANT SELECT ON masked.column_classification TO analyst;
