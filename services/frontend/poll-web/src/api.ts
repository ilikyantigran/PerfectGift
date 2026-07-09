// Network module: the only place that talks to the API Gateway.
//
// It consumes exactly the two anonymous, token-scoped routes:
//   GET  /v1/polls/token/{t}            -> load poll
//   POST /v1/polls/token/{t}/responses  -> submit answers
//
// No Authorization header is ever sent — the Poll Service validates the opaque token
// itself. Errors are mapped to typed classes so the UI can render the calm terminal
// states the spec requires (expired/invalid link, rate-limited).
//
// A `fetch` implementation is injectable so this module is fully unit-testable against
// a fake, with no live backend.

import { GATEWAY_BASE_URL } from "./config";
import type { Answer, Poll, Question, QuestionType } from "./types";

export interface ApiOptions {
  baseUrl?: string;
  fetchImpl?: typeof fetch;
}

/** The link is gone: expired, revoked, unknown, or already used. Uniform 404. */
export class PollUnavailableError extends Error {
  constructor(message = "This poll link is no longer available.") {
    super(message);
    this.name = "PollUnavailableError";
  }
}

/** Too many requests for this token/IP. HTTP 429. Do not auto-retry hard. */
export class RateLimitedError extends Error {
  /** Seconds to wait, parsed from Retry-After when present. */
  readonly retryAfterSeconds?: number;
  constructor(retryAfterSeconds?: number, message = "Too many attempts. Please wait a moment and try again.") {
    super(message);
    this.name = "RateLimitedError";
    this.retryAfterSeconds = retryAfterSeconds;
  }
}

/** Any other unexpected failure (network error, 5xx, malformed body). */
export class PollApiError extends Error {
  readonly status?: number;
  constructor(message = "Something went wrong. Please try again.", status?: number) {
    super(message);
    this.name = "PollApiError";
    this.status = status;
  }
}

// --- wire shapes (snake_case, as the Gateway returns) ---

interface WireOption {
  id?: string;
  label?: string;
}

interface WireQuestion {
  id?: string;
  prompt?: string;
  type?: string | number;
  options?: WireOption[];
  required?: boolean;
}

interface WireGetPoll {
  poll_id?: string;
  title?: string;
  questions?: WireQuestion[];
}

interface WireSubmitResponse {
  ok?: boolean;
}

// Map the proto enum (string name or numeric) to our small union.
function normalizeType(t: string | number | undefined): QuestionType {
  switch (t) {
    case "QUESTION_TYPE_TEXT":
    case 1:
    case "text":
      return "text";
    case "QUESTION_TYPE_SINGLE_CHOICE":
    case 2:
    case "single_choice":
      return "single_choice";
    case "QUESTION_TYPE_MULTI_CHOICE":
    case 3:
    case "multi_choice":
      return "multi_choice";
    default:
      // Unknown/unspecified — safest fallback is a free-text box.
      return "text";
  }
}

function normalizeQuestion(q: WireQuestion): Question {
  return {
    id: q.id ?? "",
    prompt: q.prompt ?? "",
    type: normalizeType(q.type),
    options: (q.options ?? []).map((o) => ({ id: o.id ?? "", label: o.label ?? "" })),
    required: Boolean(q.required),
  };
}

function resolve(options?: ApiOptions) {
  return {
    baseUrl: (options?.baseUrl ?? GATEWAY_BASE_URL).replace(/\/+$/, ""),
    doFetch: options?.fetchImpl ?? fetch,
  };
}

function parseRetryAfter(res: Response): number | undefined {
  const raw = res.headers.get("Retry-After");
  if (!raw) return undefined;
  const n = Number(raw);
  return Number.isFinite(n) && n >= 0 ? n : undefined;
}

// Map a non-OK response to the right typed error. Shared by both calls.
function mapErrorResponse(res: Response): never {
  if (res.status === 404) throw new PollUnavailableError();
  if (res.status === 429) throw new RateLimitedError(parseRetryAfter(res));
  throw new PollApiError(`Request failed with status ${res.status}.`, res.status);
}

/**
 * Load a poll by its opaque link token.
 * @throws PollUnavailableError on 404, RateLimitedError on 429, PollApiError otherwise.
 */
export async function fetchPoll(token: string, options?: ApiOptions): Promise<Poll> {
  const { baseUrl, doFetch } = resolve(options);
  const url = `${baseUrl}/v1/polls/token/${encodeURIComponent(token)}`;

  let res: Response;
  try {
    res = await doFetch(url, {
      method: "GET",
      headers: { Accept: "application/json" },
    });
  } catch (e) {
    throw new PollApiError(e instanceof Error ? e.message : "Network request failed.");
  }

  if (!res.ok) mapErrorResponse(res);

  let body: WireGetPoll;
  try {
    body = (await res.json()) as WireGetPoll;
  } catch {
    throw new PollApiError("The server sent an unreadable response.");
  }

  return {
    pollId: body.poll_id ?? "",
    title: body.title ?? "",
    questions: (body.questions ?? []).map(normalizeQuestion),
  };
}

/**
 * Submit the Subject's answers for a poll token.
 * @throws PollUnavailableError on 404, RateLimitedError on 429, PollApiError otherwise.
 */
export async function submitResponse(
  token: string,
  answers: Answer[],
  options?: ApiOptions,
): Promise<void> {
  const { baseUrl, doFetch } = resolve(options);
  const url = `${baseUrl}/v1/polls/token/${encodeURIComponent(token)}/responses`;

  let res: Response;
  try {
    res = await doFetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json", Accept: "application/json" },
      body: JSON.stringify({ answers }),
    });
  } catch (e) {
    throw new PollApiError(e instanceof Error ? e.message : "Network request failed.");
  }

  if (!res.ok) mapErrorResponse(res);

  // Body is { ok: true }; treat a missing/false ok as a soft failure.
  try {
    const body = (await res.json()) as WireSubmitResponse;
    if (body.ok === false) {
      throw new PollApiError("The poll could not be submitted.");
    }
  } catch (e) {
    // A 2xx with an empty/unparseable body still counts as success; only rethrow
    // our own explicit PollApiError.
    if (e instanceof PollApiError) throw e;
  }
}
