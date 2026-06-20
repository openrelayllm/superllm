ALTER TABLE admin_plus_extension_tasks
    DROP CONSTRAINT IF EXISTS admin_plus_extension_tasks_type_check;

ALTER TABLE admin_plus_extension_tasks
    ADD CONSTRAINT admin_plus_extension_tasks_type_check
    CHECK (type IN (
        'fetch_rates',
        'fetch_groups',
        'fetch_balance',
        'fetch_promotions',
        'export_bills',
        'fetch_health',
        'capture_supplier_session'
    ));
