# EDI Simulator — Decision Journal

## Feb 15, 2026 — 14:30-15:45 UTC

### Session 1: Project Foundation & Scalability Review

**Decisions Locked In:**

| #     | Decision                                                  | Rationale                                                                       | Scalability Impact                                                                     |
| ----- | --------------------------------------------------------- | ------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------- |
| D-001 | Use **Go** (no frameworks) + stdlib `net/http`            | Minimal overhead, forces architectural clarity, microservices concurrency model | Phase 1→6: Zero language/framework migration risk                                      |
| D-002 | **RabbitMQ** over Kafka for Phase 1-3                     | Simpler learning curve, easier broker debugging, perfect for Phase 1            | Phase 3+: Add retry logic, DLQ via config; no code changes                             |
| D-003 | **PostgreSQL** (not SQLite)                               | Production-realistic, scales to later phases, handles JSONB                     | Phase 5+: Seamlessly add transformation metadata                                       |
| D-004 | **Repository interface pattern** for storage access       | No DB lock-in, swap implementations without touching business logic             | Phase 4+: Add caching layer; Phase 6+: monitoring wrapper                              |
| D-005 | **Generic Message model** with JSONB `Metadata` field     | Support X12, EDIFACT, XML, JSON without schema migrations                       | Phase 2+: New format = new `format` value; Phase 5+: transformation data in `metadata` |
| D-006 | **Always publish to RabbitMQ**, even Phase 1              | Sender publishes → Broker → Receiver (no direct DB writes by sender)            | Phase 3+: Retry/DLQ/routing added without sender/receiver changes                      |
| D-007 | **API versioning from day one** (`/api/v1/*`)             | Prevent client breaking changes as features evolve                              | Phase 4+: New endpoints (`/api/v2/transform`) coexist safely                           |
| D-008 | **Configuration via environment** (`.env`), not hardcoded | Deployments don't require code rebuilds                                         | Phase 6+: Swap brokers/DBs by changing ENV                                             |
| D-009 | **SQL Repository methods** + parameterized queries        | Prevents SQL injection, readable data layer                                     | Phase 4+: Add query caching; prepare for migration to schema versioning                |
| D-010 | **Middleware pattern** in HTTP handlers                   | Easy layer addition (logging, tracing, auth) without refactoring handlers       | Phase 6+: Add correlation IDs, distributed tracing without code churn                  |

### Tech Stack Confirmed

| Component | Choice                                | Free? | Rationale                               |
| --------- | ------------------------------------- | ----- | --------------------------------------- |
| Language  | Go 1.x                                | ✓     | Concurrency, performance, stdlib power  |
| HTTP      | `net/http` stdlib                     | ✓     | Zero dependencies, built-in             |
| DB        | PostgreSQL 16                         | ✓     | JSONB, reliability, realistic           |
| Broker    | RabbitMQ 3.13                         | ✓     | Simple, easy to debug, production-grade |
| Container | Docker Compose                        | ✓     | Local dev, simple orchestration         |
| ORM       | None (raw SQL + Repository interface) | ✓     | Explicit, scalable, teaches design      |

### Architecture Principles (Non-Negotiable)

- **No framework magic** — explicit error handling, clear service boundaries
- **Broker-first communication** — even hardcoded Phase 1 message goes through RabbitMQ
- **Immutable schema decisions** — JSONB metadata prevents future migrations
- **Interface-driven storage** — swap implementations without ripple effects

### Next Phase

**Phase 1** (starting today):

- Project scaffolding: `go.mod`, directory structure
- Config system: load from `.env`
- Database: PostgreSQL with generic Message schema
- Broker setup: RabbitMQ topology in docker-compose
- 3 services: API Gateway, Sender, Receiver
- One hardcoded message flows end-to-end

**No refactoring expected** until Phase 6 (observability additions).

---

**Status**: ✅ Approved — Ready for Phase 1 implementation
