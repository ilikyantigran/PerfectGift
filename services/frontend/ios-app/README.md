# PerfectGift — iOS App

The **User's (planner's) primary client**: sign in, describe an occasion, get ranked
surprise ideas, optionally create a poll for a partner, and host the Subject poll when the
phone is handed over. SwiftUI + MVVM, `async/await` + `URLSession` against the API Gateway.

Built from [`SERVICE.md`](./SERVICE.md) and the gateway contract
([`../../backend/api-gateway/openapi.yaml`](../../backend/api-gateway/openapi.yaml)).

---

## Layout

The project is split into a **buildable/testable Swift package core** and a **SwiftUI app
target**:

```
ios-app/
├── Package.swift                 # SPM: PerfectGiftKit library + test target
├── run-tests.sh                  # runs the tests (see "Testing" below)
├── Sources/PerfectGiftKit/       # platform-neutral core — builds & tests via `swift`
│   ├── Config/AppConfig.swift        # gateway base URL + client IDs (env-overridable)
│   ├── Models/                       # Codable DTOs, enums, error envelope
│   ├── Networking/                   # APIClient protocol, LiveAPIClient, transport, coding
│   ├── Auth/                         # TokenStore (Keychain), TokenProvider (refresh)
│   └── ViewModels/                   # MVVM view models (ObservableObject)
├── Tests/PerfectGiftKitTests/    # Swift Testing unit tests against a fake API/transport
└── App/                          # the iOS app target (open in Xcode)
    ├── project.yml                   # XcodeGen spec → PerfectGift.xcodeproj
    ├── PerfectGiftApp.swift          # @main App entry
    ├── AppDelegate.swift             # APNs registration + push delivery
    ├── AppEnvironment.swift          # composition root (DI)
    ├── AppRouter.swift               # universal-link + push navigation intents
    ├── Views/                        # SwiftUI screens
    └── Resources/                    # Info.plist, entitlements, Assets.xcassets
```

**Why the split?** All logic worth unit-testing (models, the API client, the view models)
lives in `PerfectGiftKit`, which contains **no SwiftUI/UIKit**. That means it builds and its
tests run from the command line against the macOS SDK — the same code the iOS app target
compiles. The SwiftUI views, `@main` entry, APNs and Sign in with Apple live in `App/` and
depend on the package.

---

## Open / build / run the app

The app target is generated with [XcodeGen](https://github.com/yonwoo9/XcodeGen) so there is
no hand-maintained `.xcodeproj` to conflict on:

```bash
brew install xcodegen         # once
cd App
xcodegen generate            # produces App/PerfectGift.xcodeproj
open PerfectGift.xcodeproj    # build & run on a simulator (⌘R)
```

Requires **Xcode 16+** and **iOS 18+** (deployment target `18.0`). (Sign in with Apple, APNs, and Universal Links need a
real signing team + provisioning; the app runs in the simulator without them for the
email-sign-in and generation flows.)

If you prefer not to use XcodeGen, create a new iOS App target in Xcode, add the `App/`
sources, add a local package dependency on this folder (`PerfectGiftKit`), and point
`INFOPLIST_FILE` / `CODE_SIGN_ENTITLEMENTS` at `App/Resources/`.

## Pointing at the gateway

The base URL defaults to the local Docker stack, `http://localhost:8080`. Override it any of
these ways (highest priority first), all read by `AppConfig`:

- Environment variable `PG_BASE_URL` (e.g. in the Xcode scheme's Run → Environment).
- `PGBaseURL` in `App/Resources/Info.plist`.
- Falls back to `AppConfig.localDefault` (`http://localhost:8080`).

Related keys: `PG_UNIVERSAL_LINK_HOST` / `PGUniversalLinkHost`, `PG_APPLE_CLIENT_ID`,
`PG_GOOGLE_CLIENT_ID`.

---

## Testing

```bash
./run-tests.sh          # or: swift test
```

Tests use **Swift Testing** (`import Testing`) and run against a **fake API client** and a
**scripted HTTP transport** — no live backend required. `run-tests.sh` is only needed on an
Xcode **Command Line Tools**-only machine (it discovers the Testing.framework paths that a
full Xcode toolchain wires automatically); under full Xcode, plain `swift test` works.

What's covered:

- **Decoding** — error envelope `{error:{code,message,details}}`, snake_case models, and the
  full-proto-name enums (`PROVIDER_APPLE`, `GENERATION_STATUS_READY`, `MODEL_TIER_OPUS`, …).
- **Submit-then-observe** — submit → poll (queued→running→ready) → ranked ideas; failure &
  retry (fresh idempotency key); push-driven advance.
- **Auth refresh-on-401** — `LiveAPIClient` transparently refreshes once and retries with the
  new token; a failed refresh clears the session.
- **View models** — sign-in (success/failure, Apple provider), occasion input, ideas
  save/rollback, poll create/share, Subject poll answer assembly + submit.

---

## Endpoints consumed (all via the gateway)

| Flow | Calls |
|---|---|
| Auth | `POST /v1/auth/signin`, `POST /v1/auth/refresh`, `POST /v1/auth/revoke`, `GET /v1/me` |
| Generate | `POST /v1/generations` → `202 {request_id}`; poll `GET /v1/generations/{id}`; `POST /v1/generations/{id}/refine` |
| Ideas | ideas from `GET /v1/generations/{id}`; `POST /v1/ideas/{id}/save` |
| Poll (owner) | `POST /v1/polls`, `GET /v1/polls/{id}/responses` |
| Poll (Subject) | `GET /v1/polls/token/{t}`, `POST /v1/polls/token/{t}/responses` |
| Reference | `GET /v1/holidays`, `GET /v1/categories` |
| Push | `POST /v1/devices` |

### Conventions honored
- `Authorization: Bearer <access_token>` on authed routes; transparent refresh + one retry on 401.
- JSON is snake_case; enums are sent by their **full proto names** (per the gateway note),
  decoded leniently so unknown values never crash the client.
- `Idempotency-Key` on `POST /v1/generations`.
- Generation is async — the UI shows progress and advances by polling or push, never blocks.
- Only the auth token is persisted (Keychain). No local DB.

---

## Notes / deviations

- **Environment limitation:** this machine has Swift Command Line Tools but not full Xcode,
  so `xcodebuild` and iOS-simulator builds can't run here. `PerfectGiftKit` (all models, the
  API client, auth, and view models) **was built and its 30 tests pass** via `swift test`.
  The SwiftUI `App/` layer is verified for structural consistency (it references only symbols
  exported by `PerfectGiftKit` and standard SwiftUI) but was not compiled here.
- **Enums:** the gateway's `openapi.yaml` shows short enum aliases (`apple`, `ready`), but the
  live backend forwards full proto names (`PROVIDER_APPLE`, `GENERATION_STATUS_READY`). The
  client sends full names and decodes both, so it is correct against the running service.
- **Google Sign-In / Apple Sign In** button plumbing is present; wiring the SDKs to obtain the
  ID token is an integration step (client IDs go in Info.plist).
