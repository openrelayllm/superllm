ALTER TABLE supplier_provision_jobs
    DROP CONSTRAINT IF EXISTS supplier_provision_jobs_type_check;

ALTER TABLE supplier_provision_jobs
    ADD CONSTRAINT supplier_provision_jobs_type_check CHECK (
        job_type IN ('sync_groups', 'provision_group_key', 'provision_all_group_keys', 'repair_binding', 'sync_supplier_costs')
    );

ALTER TABLE supplier_provision_steps
    DROP CONSTRAINT IF EXISTS supplier_provision_steps_type_check;

ALTER TABLE supplier_provision_steps
    ADD CONSTRAINT supplier_provision_steps_type_check CHECK (
        step_type IN (
            'ensure_supplier_session',
            'sync_supplier_group',
            'ensure_third_party_key',
            'ensure_sub2api_group',
            'ensure_sub2api_account',
            'upsert_admin_plus_binding',
            'enqueue_initial_collection',
            'provision_all_group_keys',
            'repair_binding',
            'sync_supplier_costs'
        )
    );
