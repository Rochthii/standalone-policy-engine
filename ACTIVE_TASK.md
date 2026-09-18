# Active Task

ID: THESIS-RQ4-FREEZE-01
Goal: Freeze one current-HEAD evidence package for thesis RQ4 and reconcile all thesis claims with reproducible functional, security and performance boundaries.
Scope: Re-run the existing Go, Odoo mTLS E2E, dense-candidate, local full-path and Odoo purchase-confirmation evidence suites; record commit, environment, commands, raw results and limits; update thesis/evidence mapping only. No AWS, OPA/Cedar comparison, multi-agent orchestration, edge or new ERP workload.
Acceptance: Seven Odoo transaction cases plus two-session retry pass; evaluator, dense, local full-path and Odoo comparison results are recorded against one commit; stale claims (27 ns, 36.8M RPS, 286.3 ns, 0.31 ms, zero-linear-scan, full-P2P and production-ready) are removed or qualified; AWS archive remains DEFERRED UNTIL 2029.
Validation: go test -count=1 ./...; go vet ./...; focused evaluator/dense benchmarks; RUN_PERF_FULL=1 full-path test; Odoo E2E and ORM benchmark; git diff --check.
Files: docs/thesis-proposal/DE_CUONG_CHI_TIET_DO_AN_TOT_NGHIEP_CHUAN_KHOA_HOC.md; docs/thesis-proposal/THESIS_SCOPE_AND_EVIDENCE_ALIGNMENT.md; docs/technical-spec/THESIS_CHAPTER_MAPPING.md; docs/technical-spec/CURRENT_STATE_AUDIT.md; docs/technical-spec/SECURITY_INVARIANTS.md; docs/technical-spec/EVALUATION_MATRIX.md; docs/technical-spec/IMPLEMENTATION_TASK_BOARD.md
