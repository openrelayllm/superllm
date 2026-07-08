ALTER TABLE admin_plus_action_recommendations
    DROP CONSTRAINT IF EXISTS admin_plus_action_recommendations_type_check;

ALTER TABLE admin_plus_action_recommendations
    ADD CONSTRAINT admin_plus_action_recommendations_type_check
    CHECK (type IN ('switch_supplier', 'pause_supplier', 'degrade_supplier', 'increase_weight', 'recharge_supplier', 'investigate_profit', 'review_credential', 'routing_refill', 'local_account_schedule_disable'));

ALTER TABLE admin_plus_action_executions
    DROP CONSTRAINT IF EXISTS admin_plus_action_executions_type_check;

ALTER TABLE admin_plus_action_executions
    ADD CONSTRAINT admin_plus_action_executions_type_check
    CHECK (action_type IN ('switch_supplier', 'pause_supplier', 'degrade_supplier', 'increase_weight', 'recharge_supplier', 'investigate_profit', 'review_credential', 'routing_refill', 'local_account_schedule_disable'));
