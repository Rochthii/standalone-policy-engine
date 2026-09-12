INSERT INTO tenants (id, name, status)
VALUES ('00000000-0000-0000-0000-000000000017', 'odoo-e2e', 'ACTIVE')
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    status = 'ACTIVE',
    updated_at = CURRENT_TIMESTAMP;

INSERT INTO policies (id, tenant_id, effect, policy_text, status, version)
VALUES
(
    '10000000-0000-0000-0000-000000000001',
    (SELECT id FROM tenants WHERE name = 'odoo-e2e'),
    'PERMIT',
    $policy$
permit(
    principal == agent:procurement_copilot,
    action == action:APPROVE_PURCHASE_ORDER,
    resource == any
)
when {
    context.tool_context == "tool:auto_confirm_po" &&
    context.amount <= 2000 &&
    context.execution_mode == "autonomous_run"
};
$policy$,
    'ACTIVE',
    1
),
(
    '10000000-0000-0000-0000-000000000002',
    (SELECT id FROM tenants WHERE name = 'odoo-e2e'),
    'FORBID',
    $policy$
forbid(
    principal == agent:procurement_copilot,
    action == action:APPROVE_PURCHASE_ORDER,
    resource == any
)
when { context.amount > 2000 }
obligation REQUIRE_HUMAN_APPROVAL "Autonomous spending requires human approval";
$policy$,
    'ACTIVE',
    1
),
(
    '10000000-0000-0000-0000-000000000003',
    (SELECT id FROM tenants WHERE name = 'odoo-e2e'),
    'FORBID',
    $policy$
forbid(
    principal == any,
    action == action:APPROVE_PURCHASE_ORDER,
    resource == any
)
when { context.delegation_chain contains resource.creator_id };
$policy$,
    'ACTIVE',
    1
)
ON CONFLICT (id) DO UPDATE SET
    tenant_id = EXCLUDED.tenant_id,
    effect = EXCLUDED.effect,
    policy_text = EXCLUDED.policy_text,
    status = 'ACTIVE',
    updated_at = CURRENT_TIMESTAMP;

UPDATE tenants SET revision = revision + 1, updated_at = CURRENT_TIMESTAMP
WHERE name = 'odoo-e2e';

SELECT pg_notify(
    'policy_events',
    json_build_object(
        'tenant_id', (SELECT id FROM tenants WHERE name = 'odoo-e2e'),
        'policy_id', 'odoo-e2e-seed',
        'action', 'UPDATE',
        'revision', (SELECT revision FROM tenants WHERE name = 'odoo-e2e')
    )::text
);
