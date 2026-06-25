ALTER TABLE admin_plus_extension_tasks
    ALTER COLUMN supplier_id DROP NOT NULL;

ALTER TABLE admin_plus_supplier_registration_credentials
    ALTER COLUMN supplier_id DROP NOT NULL;

ALTER TABLE admin_plus_extension_tasks
    DROP CONSTRAINT IF EXISTS admin_plus_extension_tasks_supplier_required_check;

ALTER TABLE admin_plus_extension_tasks
    ADD CONSTRAINT admin_plus_extension_tasks_supplier_required_check
    CHECK (supplier_id IS NOT NULL OR type = 'register_supplier_account');
