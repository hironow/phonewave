# 0005. phonewave relay-preserve actor type & deferred daemon-emit injection (Phase β-4)

**Date:** 2026-05-09
**Status:** Accepted
**Linked ADR (gateway):** runops-gateway docs/adr/0035-ai-agent-cannot-approve-ai-agent.md (architectural pin)
**Linked ADR (gateway):** runops-gateway docs/adr/0036-phase-4a-approval-actor-validation.md (effective rule)
**Linked ADR (gateway):** runops-gateway docs/adr/0037-producer-actor-classification.md (4 axes 契約、 Axis 3 dual-actor)
**Linked ADR (sightjack):** sightjack docs/adr/0017-producer-actor-type-injection.md (Phase β-1 pilot)
**Linked issue:** refs/docs/issues/0011-runops-gateway-ai-agent-identity-4-eyes.md (tap mono-tree)

## Context

gateway-side ADR 0035/0036/0037 で AI agent vs human approver の architectural pin が確立した。 sightjack/paintress/amadeus 3 ツールは ADR 0017 (Phase β-1〜β-3) で `internal/platform/actortype/` substrate を介して emit-time に `metadata.requester_actor_type` を injection 済 (PR #205/#207/#208 着地)。

phonewave は他 3 ツールと **性格が異なる**:

1. **relay-only daemon** — phonewave は他ツールが outbox に書いた D-Mail を読み取り、 routing して target inbox に配達する courier。 D-Mail を **新規生成する経路は現状存在しない** (= `MarshalDMail` / `Stage→Flush` で自前 D-Mail を outbox 化するコードパスが無い)。
2. **byte-for-byte relay** — `Deliver` / `DeliverData` は input data を `ParseDMailFrontmatter` で読むが、 metadata 加工は **一切しない**。 `StageDelivery` で SQLite に raw bytes を格納し、 `FlushDeliveries` で `atomicWrite` により target path に **byte-for-byte** 書き出す。 `metadata` map はルーティング判断 (= `harness.SelectDeliveryInboxes`) にのみ使用される。
3. **dual-actor 候補** — 将来 daemon 自身が D-Mail を emit する経路 (= courier alert / 状態通知) を追加する場合、 そのパスは ADR 0017 と同等の producer rollout 対象になる。 現状その経路は実装されていないが、 ADR 0037 §Axis 3 dual-actor 問題が指す将来の拡張ポイントである。

つまり phonewave は ADR 0017 の helper を「**挿す場所がない**」 状態であり、 sightjack/paintress/amadeus と同じ `ComposeDMail` / `SendDMail` / `applyDMailGenerated` 相当の emit entry が存在しない。 一方で relay-preserve invariant (= 他ツール emit の `metadata.requester_actor_type` を上書き / 削除しない) が壊れると、 gateway 側 4-eyes flow で AI/human classification が崩れる。

ADR 0037 §Migration の path-split に基づく phonewave の責務:

- **HIGH path** (= convergence approval): producer (= sightjack/paintress/amadeus) で 既に inject 済の `metadata.requester_actor_type` を **relay 経路で保全する**。 上書きや削除があれば即 fail-closed の仮定が崩れる。
- **非 HIGH path** (= dispatch / canary): 同上。 migration window フォールバックが正しく働くために、 producer 入力を改変しない契約を保持する。

## Decision

### 1. relay-preserve invariant の保証 (現状維持 + regression test で pin)

phonewave の `Deliver` / `DeliverData` は **既存実装上 既に relay-preserve を達成している**:

- `ParseDMailFrontmatter` で metadata を読み取るが、 元 data は byte-for-byte 保持される
- `StageDelivery(ctx, dmailPath, data, targets)` で SQLite に raw bytes を格納
- `FlushDeliveries` で `atomicWrite(target, data)` により target path へ byte-for-byte 書き出し
- metadata に対する書込 / 削除 / canonicalization 経路は **ゼロ**

本 ADR では現状実装を **invariant test で pin** し、 将来の regression を防ぐ:

- `metadata.requester_actor_type` を持つ D-Mail を relay すると、 target に書き出された data の同 metadata がビット完全一致
- `metadata.requester_actor_source` / `metadata.initiating_actor_type` も同様に保全
- producer が値を入れていない場合 (= legacy compat) も target に出る data は同様に値が無い (= byte-identical legacy)

### 2. daemon-generated emit は現状 N/A、 将来追加時の方針

現状 phonewave に D-Mail 新規 emit 経路は無い。 将来 `feat(phonewave)` で daemon 自身が D-Mail (= courier alert / status notification) を発信する経路を追加する場合は、 以下を必須とする:

- `internal/platform/actortype/` に sightjack/paintress/amadeus と **byte-identical** な helper を配置 (= ADR 0017 の copy-sync substrate に phonewave も加わる)
- 新規 emit entry (= 仮 `EmitCourierAlert` 等) で `actortype.InjectActorType(metadata)` を呼ぶ
- daemon 起動仕様で `RUNOPS_ACTOR_TYPE=workspace-daemon` を必ず set する責務 (= systemd unit / launcher 構築仕様、 別 spec)
- daemon が他ツールから委譲された処理を発信する場合は `RUNOPS_INITIATING_ACTOR_TYPE` で initiating actor を carry (= gateway ADR 0036 effective_requester_actor_type rule 対応)

本 ADR では実装を伴わず、 将来追加時の **設計契約** として明示する。 追加実装は別 ADR (= 仮 0006) で取り扱う。

### 3. relay 経路の actor type 上書き禁止 (= structural invariant)

phonewave の relay 経路 (= `Deliver` / `DeliverData` / `StageDelivery` / `FlushDeliveries`) において、 以下を禁止:

- `metadata["requester_actor_type"] = ...` の書込 (= 上書き)
- `delete(metadata, "requester_actor_type")` (= 削除)
- frontmatter 再生成による implicit な値書き換え

将来 daemon-emit を追加する際は **emit 経路と relay 経路を構造的に分離する** (= 別関数 / 別 entry) ことで、 relay-preserve invariant を維持する。 emit 経路は新たに `actortype.InjectActorType` を呼ぶが、 relay 経路はそれを呼ばない。

semgrep rule で relay 経路の上書き検出は **将来の追加 PR** で扱う (= daemon-emit 追加時に同時導入)。 現状は invariant test で十分。

### 4. ADR 0037 Axis 3 dual-actor との整合

ADR 0037 §Axis 3 (dual-actor: emit-side actor vs relay-side carrier) で「phonewave のような relay daemon は relay-preserve を保証せよ」 と pin されている。 本 ADR はその要件を phonewave 側で具体化したもの:

- **emit-side**: producer (sightjack/paintress/amadeus) が ADR 0017 helper で inject 済
- **relay-side**: phonewave は byte-for-byte preserve (本 ADR §1)
- **daemon-emit-side**: 現状 N/A、 将来 ADR 0017 helper 流用 (本 ADR §2)

## Enforcement inventory

ADR 0037 §Enforcement inventory framework に基づく path 網羅:

### Entry points (phonewave producer/relay caller)

- `internal/session/deliver.go` `Deliver` (= file path で受ける relay 入口)
- `internal/session/deliver.go` `DeliverData` (= bytes で受ける relay 内部実装、 全 relay の最終通過点)
- `internal/session/delivery_store.go` `SQLiteDeliveryStore.StageDelivery` (= data を SQLite に格納、 加工なし)
- `internal/session/delivery_store.go` `SQLiteDeliveryStore.FlushDeliveries` (= `atomicWrite` で target に byte-for-byte 書込)
- **(将来) `internal/session/<emit>.go`** — daemon-generated emit を追加する場合の新 entry。 現状不在。

### Persistent / carried data needed at each enforcement point

- `metadata.requester_actor_type` (string, 4 canonical values) — relay 経路で **改変禁止**
- `metadata.requester_actor_source` (string) — relay 経路で **改変禁止**
- `metadata.initiating_actor_type` (string) — relay 経路で **改変禁止**
- 上記 3 keys はすべて producer 側で inject 済を前提とする (= ADR 0017 で sightjack/paintress/amadeus 着地済)

### Bypass candidates

- relay 経路で metadata を canonicalization / re-marshal して値が消える → 現状 `data` は parse のみで marshal していないため発生しないが、 invariant test で pin
- relay 経路で `metadata["requester_actor_type"] = ""` のように explicit overwrite → semgrep rule で検出 (将来追加)
- producer から actor type 抜きの D-Mail を受信 → 本 ADR スコープ外 (= producer 側 ADR 0017 の責務、 gateway 側 path-split で処理)
- 将来 daemon-emit を追加した際に helper を呼び忘れる → ADR 0017 helper をそのまま使う設計契約 (本 ADR §2)、 追加 PR で semgrep rule pin
- relay と emit の経路混線 (= 同一関数で両モードを扱う) → 本 ADR §3 で構造分離を pin、 違反は code review で検出

### Tests proving coverage

| Test | Layer | Verifies |
|---|---|---|
| `TestDeliverData_PreservesActorType_HumanOperator` | session integration | producer emit `requester_actor_type=human-operator` を含む D-Mail relay → target data の同 metadata がビット完全一致 |
| `TestDeliverData_PreservesActorType_AIAgent` | session integration | 同上、 `ai-agent` で確認 |
| `TestDeliverData_PreservesActorType_DaemonWithInitiating` | session integration | `requester_actor_type=workspace-daemon` + `initiating_actor_type=human-operator` の両キーを保全 |
| `TestDeliverData_PreservesActorSource` | session integration | `requester_actor_source=env` の値を改変せずに保全 |
| `TestDeliverData_NoActorType_LegacyCompat` | session integration | producer が actor type を入れていない (= legacy compat) D-Mail を relay → target data に actor type 関連 keys が **新規挿入されない** (= legacy byte-identical) |
| `TestDeliverData_ByteIdentity` | session integration | input bytes と target bytes が `bytes.Equal` で完全一致 (= raw bytes preserve の最強形) |

## Consequences

### Positive

- relay-preserve invariant が test で pin され、 将来 regression を防ぐ
- gateway ADR 0035/0036/0037 system-level invariant が phonewave 経由でも貫通 (= AI/human classification が producer→relay→gateway で carry-through)
- daemon-emit を将来追加する際の設計契約 (= ADR 0017 helper 流用) が明示され、 phonewave だけ別 substrate にならない
- ADR 0037 §Axis 3 dual-actor の relay-side 責務が phonewave 側で具体化される

### Negative

- 将来 daemon-emit を追加する際に ADR 0017 helper の copy-sync を 4 ツール目として整備する必要がある (= 本 ADR §2 で予告済、 別 ADR で扱う)
- relay-preserve invariant test はあくまで構造的 pin、 producer 側で actor type が入っていない場合は relay 後も入らない (= 本 ADR スコープ外、 producer 側 ADR 0017 の責務)

### Neutral

- 現状 phonewave に actortype helper を **置かない** ことで、 ADR 0017 の copy-sync 対象は当面 3 ツール (sightjack/paintress/amadeus)。 4 ツール目への展開は将来 daemon-emit 追加時。
- semgrep rule (relay 経路での actor type 上書き禁止) は本 ADR では追加せず、 将来 daemon-emit と同時に導入する (= 単独で導入しても false-positive リスクと検査対象が乖離する懸念)。

## References

- gateway ADR 0035: AI agent cannot approve AI agent (architectural pin)
- gateway ADR 0036: Phase 4a approval actor validation (effective_requester_actor_type rule)
- gateway ADR 0037: producer-side actor classification (4 axes、 §Axis 3 dual-actor、 §Enforcement inventory framework)
- sightjack ADR 0017: producer-side `requester_actor_type` injection (Phase β-1 pilot、 helper substrate canonical)
- paintress / amadeus: ADR 0017 byte-identical copy-sync (Phase β-2 / β-3、 PR #207 / #208)
- 4 ツール copy-sync 原則: shared ADR S0037 (substrate canonical lock)
- phonewave Deliver entry: `internal/session/deliver.go`
- phonewave Stage/Flush: `internal/session/delivery_store.go`
