# PerfectGift — Android app

The User's (planner's) primary Android client — native **Kotlin + Jetpack Compose**,
parity with the iOS app. Sign in, describe an occasion (holiday, budget, free-form
preferences), optionally create a poll for the partner, and receive **ranked surprise
ideas**. Generation is asynchronous (~3–15 s), so the app follows the defining
**submit-then-observe** pattern: POST the request, show progress, poll status (or receive
a push), and render ideas when ready. It also hosts the **Subject** poll flow natively on
a handed-over phone or via a shared App Link.

It is a **pure client of the API Gateway** — REST/JSON only, no direct backend contact.
See [`SERVICE.md`](SERVICE.md) for the client spec and the gateway's `openapi.yaml` for
the contract this app consumes.

---

## Stack

| Concern | Choice |
|---|---|
| UI | Jetpack Compose, Material 3, MVVM |
| Navigation | Navigation-Compose (single-activity) |
| Networking | Retrofit + OkHttp + Coroutines/Flow |
| JSON | Gson (camelCase ⇄ snake_case via naming policy) |
| Auth token storage | **DataStore** (Preferences) — the *only* persisted state |
| Async state | `StateFlow` per-screen ViewModels (no shared global store) |
| Push | Firebase Cloud Messaging (FCM) |
| Deep links | App Links (`https://perfectgift.app/p/{token}`) |
| DI | Hand-rolled `AppContainer` (no DI framework) |

Requirements: **JDK 17**, the **Android SDK** (compileSdk/targetSdk 34, minSdk 26), and
Android Studio (Koala or newer recommended). Toolchain versions are pinned in
[`gradle/libs.versions.toml`](gradle/libs.versions.toml) (AGP 8.5.2, Kotlin 1.9.24,
Compose BOM 2024.06.00, Gradle 8.9).

---

## Open / build / run

```bash
# From services/frontend/android-app/
./gradlew assembleDebug      # build the debug APK
./gradlew installDebug       # build + install on a running emulator/device
./gradlew test               # run the JVM unit tests (no device needed)
```

Or open the `services/frontend/android-app/` folder in **Android Studio** and press Run.

> **Gradle wrapper JAR.** This tree ships `gradlew`, `gradlew.bat` and
> `gradle/wrapper/gradle-wrapper.properties`, but **not** the binary
> `gradle/wrapper/gradle-wrapper.jar`. Android Studio regenerates it automatically on
> first sync; from the CLI, run `gradle wrapper --gradle-version 8.9` once (with a
> system Gradle) to materialize it. This is the only missing artifact.

---

## Gateway base URL (per build variant)

The base URL is a `BuildConfig` field set per build type in
[`app/build.gradle.kts`](app/build.gradle.kts):

| Variant | `GATEWAY_BASE_URL` | When |
|---|---|---|
| **debug** | `http://10.0.2.2:8080/` | Local Docker stack from the Android **emulator** |
| **release** | `https://api.perfectgift.app/` | Production |

> **Emulator note.** The Android emulator cannot reach the host's `localhost` directly.
> The special alias **`10.0.2.2`** maps to the host loopback, so with the local Docker
> stack running on `localhost:8080` the debug build reaches it at `http://10.0.2.2:8080/`.
> - On a **physical device** on the same LAN, change the debug URL to your machine's LAN
>   IP (e.g. `http://192.168.1.50:8080/`) and allow cleartext for that host.
> - Cleartext HTTP is used only for local dev; production is HTTPS.

To point at a different backend, edit the `buildConfigField("String", "GATEWAY_BASE_URL", …)`
lines and rebuild.

---

## How to test

```bash
./gradlew test
```

JVM unit tests only — **no live backend, no device, no Android SDK emulation required**.
They run against a fake API and an in-process `MockWebServer`:

- **`GenerationFlowTest`** — the submit-then-observe flow end-to-end through
  `GenerationViewModel`: queued → running → ready renders ranked ideas; a `failed` status
  degrades gracefully; `observeGeneration` emits each status and terminates. Also asserts
  an `Idempotency-Key` is sent on submit.
- **`TokenRefreshTest`** — the real OkHttp `TokenAuthenticator` against `MockWebServer`: a
  `401` triggers a `/v1/auth/refresh` and a single retry with the rotated access token;
  refresh failure signs the session out.
- **`ErrorEnvelopeTest`** — decoding of the `{ error: { code, message, details } }`
  envelope, with a friendly fallback for non-envelope bodies.

---

## Sign-in providers

Email + password sign-in is wired end-to-end. **Google** (Credential Manager) and
**Apple** buttons are stubbed at the token-acquisition step: once a provider ID token is
obtained, hand it to `SignInViewModel.signInWithGoogle(idToken)` /
`signInWithApple(idToken)` — the network path is complete. Add your Google/Apple client
IDs to enable the native flows.

### Provider enum wire format

Per the client build directive, the sign-in `provider` field is serialized by its **full
protobuf name** — `PROVIDER_EMAIL` / `PROVIDER_APPLE` / `PROVIDER_GOOGLE` — via
`@SerializedName` on `AuthProvider` (`data/remote/dto/AuthDto.kt`). The gateway's current
`openapi.yaml` example instead shows the short form (`email`/`apple`/`google`). If the
deployed gateway expects the short form, change only the three `@SerializedName` values —
nothing else references the raw strings. See **Deviations** in the delivery notes.

---

## Push (FCM)

The `PerfectGiftMessagingService` and device-registration call (`POST /v1/devices`) are
implemented, and the app **compiles without** a Firebase config. To actually receive
pushes:

1. Add `app/google-services.json` (from your Firebase project).
2. Apply the Google Services plugin: add the classpath/plugin and uncomment
   `id("com.google.gms.google-services")` in `app/build.gradle.kts`.

---

## Project layout

```
app/src/main/java/com/perfectgift/android/
  data/
    auth/        TokenStore (DataStore), SessionManager
    remote/      GatewayApi (Retrofit), DTOs, AuthInterceptor,
                 TokenAuthenticator (401→refresh→retry), NetworkModule, ApiResult
    repository/  PerfectGiftRepository (interface) + Impl
  di/            AppContainer (manual DI)
  ui/
    auth/        SignInScreen + SignInViewModel
    generation/  Occasion / Generating / Ideas screens + GenerationViewModel
    poll/        PollCreateScreen + PollViewModel (owner: create + share + responses)
    subject/     SubjectPollScreen + SubjectPollViewModel (handed-over phone / App Link)
    nav/         NavGraph, Routes
    common/      shared Compose components
    theme/       Material 3 theme
  push/          PerfectGiftMessagingService (FCM)
  MainActivity, PerfectGiftApp
app/src/test/java/…  fake API, fake token store, ViewModel/authenticator/error tests
```
