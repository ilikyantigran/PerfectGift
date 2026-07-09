import { useEffect, useMemo, useState, type ReactNode } from "react";
import {
  fetchPoll,
  submitResponse,
  PollUnavailableError,
  RateLimitedError,
} from "./api";
import type { Poll } from "./types";
import { currentToken } from "./token";
import { QuestionField } from "./QuestionField";
import {
  buildAnswers,
  emptyValue,
  findMissingRequired,
  type ValueMap,
} from "./answers";

// The whole app is one small state machine over these phases.
type Phase =
  | { kind: "loading" }
  | { kind: "no-token" }
  | { kind: "unavailable" } // expired / invalid / revoked link (uniform 404)
  | { kind: "rate-limited" }
  | { kind: "error"; message: string }
  | { kind: "ready"; poll: Poll }
  | { kind: "submitting"; poll: Poll }
  | { kind: "thank-you" };

export function App() {
  const [phase, setPhase] = useState<Phase>({ kind: "loading" });
  const [values, setValues] = useState<ValueMap>({});
  const [missing, setMissing] = useState<Set<string>>(new Set());
  const [submitError, setSubmitError] = useState<string | null>(null);

  const token = useMemo(() => currentToken(), []);

  // Load the poll once on mount.
  useEffect(() => {
    if (!token) {
      setPhase({ kind: "no-token" });
      return;
    }
    let cancelled = false;
    fetchPoll(token)
      .then((poll) => {
        if (cancelled) return;
        const init: ValueMap = {};
        for (const q of poll.questions) init[q.id] = emptyValue();
        setValues(init);
        setPhase({ kind: "ready", poll });
      })
      .catch((err) => {
        if (cancelled) return;
        setPhase(mapError(err));
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  async function handleSubmit(poll: Poll) {
    setSubmitError(null);
    const stillMissing = findMissingRequired(poll.questions, values);
    if (stillMissing.length > 0) {
      setMissing(new Set(stillMissing));
      // Bring the first missing field into view.
      document.getElementById(stillMissing[0])?.scrollIntoView({ behavior: "smooth", block: "center" });
      return;
    }
    setMissing(new Set());
    setPhase({ kind: "submitting", poll });
    try {
      await submitResponse(token!, buildAnswers(poll.questions, values));
      setPhase({ kind: "thank-you" });
    } catch (err) {
      const mapped = mapError(err);
      if (mapped.kind === "ready" || mapped.kind === "submitting") {
        // never happens, but keeps the type exhaustive
        setPhase({ kind: "error", message: "Unexpected state." });
      } else if (mapped.kind === "error") {
        // stay on the form so the Subject can retry
        setSubmitError(mapped.message);
        setPhase({ kind: "ready", poll });
      } else {
        setPhase(mapped);
      }
    }
  }

  switch (phase.kind) {
    case "loading":
      return (
        <Screen>
          <p className="muted">Loading…</p>
        </Screen>
      );

    case "no-token":
      return (
        <Screen>
          <h1>Nothing to see here</h1>
          <p className="muted">This page needs a poll link to open. Please use the link you were sent.</p>
        </Screen>
      );

    case "unavailable":
      return (
        <Screen>
          <h1>This link is no longer available</h1>
          <p className="muted">
            It may have expired or already been used. Ask the person who sent it for a fresh link.
          </p>
        </Screen>
      );

    case "rate-limited":
      return (
        <Screen>
          <h1>Just a moment</h1>
          <p className="muted">
            There have been a lot of attempts on this link. Please wait a little while, then reopen it.
          </p>
        </Screen>
      );

    case "error":
      return (
        <Screen>
          <h1>Something went wrong</h1>
          <p className="muted">{phase.message}</p>
        </Screen>
      );

    case "thank-you":
      return (
        <Screen>
          <div className="checkmark" aria-hidden="true">✓</div>
          <h1>Thank you!</h1>
          <p className="muted">Your answers were sent. You can close this page now.</p>
        </Screen>
      );

    case "ready":
    case "submitting": {
      const poll = phase.poll;
      const submitting = phase.kind === "submitting";
      return (
        <Screen>
          <h1 className="poll__title">{poll.title}</h1>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              if (!submitting) void handleSubmit(poll);
            }}
          >
            {poll.questions.map((q) => (
              <QuestionField
                key={q.id}
                question={q}
                value={values[q.id] ?? emptyValue()}
                invalid={missing.has(q.id)}
                onChange={(next) => setValues((prev) => ({ ...prev, [q.id]: next }))}
              />
            ))}

            {submitError && (
              <p className="form__error" role="alert">
                {submitError}
              </p>
            )}

            <button type="submit" className="submit" disabled={submitting}>
              {submitting ? "Sending…" : "Submit"}
            </button>
          </form>
        </Screen>
      );
    }
  }
}

function mapError(err: unknown): Phase {
  if (err instanceof PollUnavailableError) return { kind: "unavailable" };
  if (err instanceof RateLimitedError) return { kind: "rate-limited" };
  const message = err instanceof Error ? err.message : "Please try again.";
  return { kind: "error", message };
}

function Screen({ children }: { children: ReactNode }) {
  return (
    <main className="screen">
      <div className="card">{children}</div>
    </main>
  );
}
